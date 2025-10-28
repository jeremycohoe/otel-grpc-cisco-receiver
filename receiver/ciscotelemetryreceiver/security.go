package ciscotelemetryreceiver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// SecurityManager manages security features for the gRPC receiver
type SecurityManager struct {
	config      *SecurityConfig
	tlsConfig   *TLSConfig
	rateLimiter *RateLimiter
	logger      *zap.Logger
}

// RateLimiter implements per-client rate limiting
type RateLimiter struct {
	limiters        map[string]*rate.Limiter
	mu              sync.RWMutex
	requestsPerSec  rate.Limit
	burstSize       int
	cleanupInterval time.Duration
	cleanupTicker   *time.Ticker
	done            chan bool
}

// NewSecurityManager creates a new SecurityManager
func NewSecurityManager(securityConfig *SecurityConfig, tlsConfig *TLSConfig, logger *zap.Logger) *SecurityManager {
	sm := &SecurityManager{
		config:    securityConfig,
		tlsConfig: tlsConfig,
		logger:    logger,
	}

	// Initialize rate limiter if enabled
	if securityConfig.RateLimiting.Enabled {
		sm.rateLimiter = NewRateLimiter(
			rate.Limit(securityConfig.RateLimiting.RequestsPerSecond),
			securityConfig.RateLimiting.BurstSize,
			securityConfig.RateLimiting.CleanupInterval,
		)
	}

	return sm
}

// NewRateLimiter creates a new RateLimiter
func NewRateLimiter(requestsPerSec rate.Limit, burstSize int, cleanupInterval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limiters:        make(map[string]*rate.Limiter),
		requestsPerSec:  requestsPerSec,
		burstSize:       burstSize,
		cleanupInterval: cleanupInterval,
		done:            make(chan bool),
	}

	// Start cleanup goroutine
	rl.cleanupTicker = time.NewTicker(cleanupInterval)
	go rl.cleanup()

	return rl
}

// Allow checks if the request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.requestsPerSec, rl.burstSize)
		rl.limiters[ip] = limiter
	}

	return limiter.Allow()
}

// cleanup removes unused rate limiters
func (rl *RateLimiter) cleanup() {
	for {
		select {
		case <-rl.cleanupTicker.C:
			rl.mu.Lock()
			// Remove limiters that haven't been used recently
			// For simplicity, we remove all limiters periodically
			// In production, you might want to track last access times
			rl.limiters = make(map[string]*rate.Limiter)
			rl.mu.Unlock()
		case <-rl.done:
			rl.cleanupTicker.Stop()
			return
		}
	}
}

// Stop stops the rate limiter cleanup
func (rl *RateLimiter) Stop() {
	close(rl.done)
}

// CreateTLSConfig creates a TLS configuration from the TLS config
func (sm *SecurityManager) CreateTLSConfig() (*tls.Config, error) {
	if !sm.tlsConfig.Enabled {
		return nil, nil
	}

	// Load server certificate and key
	cert, err := tls.LoadX509KeyPair(sm.tlsConfig.CertFile, sm.tlsConfig.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   sm.getTLSVersion(sm.tlsConfig.MinVersion, tls.VersionTLS12),
		MaxVersion:   sm.getTLSVersion(sm.tlsConfig.MaxVersion, tls.VersionTLS13),
	}

	// Configure client authentication
	if sm.tlsConfig.ClientAuthType != "" {
		clientAuthType, err := sm.getClientAuthType(sm.tlsConfig.ClientAuthType)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientAuth = clientAuthType

		// Load CA certificates if provided
		if sm.tlsConfig.CAFile != "" {
			caCert, err := ioutil.ReadFile(sm.tlsConfig.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA file: %w", err)
			}

			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}
			tlsConfig.ClientCAs = caCertPool
		}
	}

	// Configure cipher suites if specified
	if len(sm.tlsConfig.CipherSuites) > 0 {
		cipherSuites, err := sm.parseCipherSuites(sm.tlsConfig.CipherSuites)
		if err != nil {
			return nil, err
		}
		tlsConfig.CipherSuites = cipherSuites
	}

	// Configure curve preferences if specified
	if len(sm.tlsConfig.CurvePreferences) > 0 {
		curves, err := sm.parseCurvePreferences(sm.tlsConfig.CurvePreferences)
		if err != nil {
			return nil, err
		}
		tlsConfig.CurvePreferences = curves
	}

	// Set other security options
	tlsConfig.InsecureSkipVerify = sm.tlsConfig.InsecureSkipVerify
	tlsConfig.PreferServerCipherSuites = true

	return tlsConfig, nil
}

// getTLSVersion converts string version to TLS version constant
func (sm *SecurityManager) getTLSVersion(version string, defaultVersion uint16) uint16 {
	if version == "" {
		return defaultVersion
	}

	versionMap := map[string]uint16{
		"1.0": tls.VersionTLS10,
		"1.1": tls.VersionTLS11,
		"1.2": tls.VersionTLS12,
		"1.3": tls.VersionTLS13,
	}

	if v, exists := versionMap[version]; exists {
		return v
	}

	return defaultVersion
}

// getClientAuthType converts string auth type to tls.ClientAuthType
func (sm *SecurityManager) getClientAuthType(authType string) (tls.ClientAuthType, error) {
	authTypeMap := map[string]tls.ClientAuthType{
		"NoClientCert":               tls.NoClientCert,
		"RequestClientCert":          tls.RequestClientCert,
		"RequireAnyClientCert":       tls.RequireAnyClientCert,
		"VerifyClientCertIfGiven":    tls.VerifyClientCertIfGiven,
		"RequireAndVerifyClientCert": tls.RequireAndVerifyClientCert,
	}

	if authType, exists := authTypeMap[authType]; exists {
		return authType, nil
	}

	return tls.NoClientCert, fmt.Errorf("invalid client auth type: %s", authType)
}

// parseCipherSuites parses cipher suite names to constants
func (sm *SecurityManager) parseCipherSuites(suiteNames []string) ([]uint16, error) {
	// This is a simplified implementation. In practice, you'd want to support
	// a comprehensive list of cipher suites
	suiteMap := map[string]uint16{
		"TLS_AES_128_GCM_SHA256":                tls.TLS_AES_128_GCM_SHA256,
		"TLS_AES_256_GCM_SHA384":                tls.TLS_AES_256_GCM_SHA384,
		"TLS_CHACHA20_POLY1305_SHA256":          tls.TLS_CHACHA20_POLY1305_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	}

	var suites []uint16
	for _, name := range suiteNames {
		if suite, exists := suiteMap[name]; exists {
			suites = append(suites, suite)
		} else {
			return nil, fmt.Errorf("unknown cipher suite: %s", name)
		}
	}

	return suites, nil
}

// parseCurvePreferences parses curve preference names to constants
func (sm *SecurityManager) parseCurvePreferences(curveNames []string) ([]tls.CurveID, error) {
	curveMap := map[string]tls.CurveID{
		"CurveP256": tls.CurveP256,
		"CurveP384": tls.CurveP384,
		"CurveP521": tls.CurveP521,
		"X25519":    tls.X25519,
	}

	var curves []tls.CurveID
	for _, name := range curveNames {
		if curve, exists := curveMap[name]; exists {
			curves = append(curves, curve)
		} else {
			return nil, fmt.Errorf("unknown curve: %s", name)
		}
	}

	return curves, nil
}

// CreateSecurityInterceptor creates a gRPC interceptor for security enforcement
func (sm *SecurityManager) CreateSecurityInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Get client IP
		clientIP, err := sm.getClientIP(ctx)
		if err != nil {
			sm.logger.Warn("Failed to get client IP", zap.Error(err))
			clientIP = "unknown"
		}

		// Check IP allowlist if configured
		if len(sm.config.AllowedClients) > 0 && !sm.isIPAllowed(clientIP) {
			sm.logger.Warn("Client IP not in allowlist", zap.String("client_ip", clientIP))
			return nil, fmt.Errorf("client IP not allowed")
		}

		// Apply rate limiting if enabled
		if sm.rateLimiter != nil && !sm.rateLimiter.Allow(clientIP) {
			sm.logger.Warn("Rate limit exceeded", zap.String("client_ip", clientIP))
			return nil, fmt.Errorf("rate limit exceeded")
		}

		sm.logger.Debug("Security check passed",
			zap.String("client_ip", clientIP),
			zap.String("method", info.FullMethod))

		return handler(ctx, req)
	}
}

// getClientIP extracts the client IP from the gRPC context
func (sm *SecurityManager) getClientIP(ctx context.Context) (string, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("no peer information in context")
	}

	if peer.Addr == nil {
		return "", fmt.Errorf("no address in peer information")
	}

	// Extract IP from address
	host, _, err := net.SplitHostPort(peer.Addr.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse peer address: %w", err)
	}

	return host, nil
}

// isIPAllowed checks if the given IP is in the allowlist
func (sm *SecurityManager) isIPAllowed(clientIP string) bool {
	for _, allowedIP := range sm.config.AllowedClients {
		// Support CIDR notation
		if strings.Contains(allowedIP, "/") {
			_, cidr, err := net.ParseCIDR(allowedIP)
			if err != nil {
				sm.logger.Warn("Invalid CIDR in allowed_clients", zap.String("cidr", allowedIP))
				continue
			}
			ip := net.ParseIP(clientIP)
			if ip != nil && cidr.Contains(ip) {
				return true
			}
		} else {
			// Direct IP match
			if clientIP == allowedIP {
				return true
			}
		}
	}
	return false
}

// Shutdown stops the security manager
func (sm *SecurityManager) Shutdown() {
	if sm.rateLimiter != nil {
		sm.rateLimiter.Stop()
	}
}
