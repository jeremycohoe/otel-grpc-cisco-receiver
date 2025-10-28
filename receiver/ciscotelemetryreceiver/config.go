package ciscotelemetryreceiver

import (
	"fmt"
	"time"
)

// TLSConfig contains TLS configuration options
type TLSConfig struct {
	// Enabled indicates whether TLS should be enabled
	Enabled bool `mapstructure:"enabled"`

	// CertFile is the path to the TLS certificate file
	CertFile string `mapstructure:"cert_file"`

	// KeyFile is the path to the TLS private key file
	KeyFile string `mapstructure:"key_file"`

	// CAFile is the path to the TLS client CA file for mTLS
	CAFile string `mapstructure:"ca_file"`

	// InsecureSkipVerify controls whether TLS certificate verification is skipped
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`

	// ClientAuthType specifies the client authentication requirement
	// Valid values: "NoClientCert", "RequestClientCert", "RequireAnyClientCert", "VerifyClientCertIfGiven", "RequireAndVerifyClientCert"
	ClientAuthType string `mapstructure:"client_auth_type"`

	// MinVersion specifies the minimum TLS version to accept
	// Valid values: "1.0", "1.1", "1.2", "1.3"
	MinVersion string `mapstructure:"min_version"`

	// MaxVersion specifies the maximum TLS version to accept
	// Valid values: "1.0", "1.1", "1.2", "1.3"
	MaxVersion string `mapstructure:"max_version"`

	// CipherSuites specifies allowed cipher suites (TLS 1.2 and below)
	CipherSuites []string `mapstructure:"cipher_suites"`

	// CurvePreferences specifies the elliptic curves that will be used in ECDHE handshakes
	CurvePreferences []string `mapstructure:"curve_preferences"`

	// ReloadInterval specifies how often to check for certificate updates
	ReloadInterval time.Duration `mapstructure:"reload_interval"`
}

// SecurityConfig contains security hardening options
type SecurityConfig struct {
	// RateLimiting contains rate limiting configuration
	RateLimiting RateLimitingConfig `mapstructure:"rate_limiting"`

	// AllowedClients contains client IP allowlist configuration
	AllowedClients []string `mapstructure:"allowed_clients"`

	// MaxConnections is the maximum number of concurrent connections
	MaxConnections int `mapstructure:"max_connections"`

	// ConnectionTimeout is the maximum time to wait for new connections
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`

	// EnableMetrics enables security-related metrics collection
	EnableMetrics bool `mapstructure:"enable_metrics"`
}

// RateLimitingConfig contains rate limiting configuration
type RateLimitingConfig struct {
	// Enabled indicates whether rate limiting should be enabled
	Enabled bool `mapstructure:"enabled"`

	// RequestsPerSecond is the maximum number of requests per second per client
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`

	// BurstSize is the maximum burst size for rate limiting
	BurstSize int `mapstructure:"burst_size"`

	// CleanupInterval is how often to clean up rate limiter entries
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// KeepAliveConfig contains gRPC keep-alive configuration
type KeepAliveConfig struct {
	// Time is the time period for sending keep-alive pings
	Time time.Duration `mapstructure:"time"`

	// Timeout is the timeout for keep-alive ping responses
	Timeout time.Duration `mapstructure:"timeout"`
}

// YANGConfig contains YANG parser configuration
type YANGConfig struct {
	// EnableRFCParser enables the RFC 6020/7950 compliant YANG parser
	EnableRFCParser bool `mapstructure:"enable_rfc_parser"`

	// CacheModules enables caching of discovered YANG modules
	CacheModules bool `mapstructure:"cache_modules"`

	// MaxModules is the maximum number of YANG modules to cache
	MaxModules int `mapstructure:"max_modules"`
}

// Config represents the receiver config settings within the collector's config.yaml
type Config struct {
	// ListenAddress is the address:port to bind to for incoming gRPC connections
	ListenAddress string `mapstructure:"listen_address"`

	// TLS contains TLS/mTLS configuration
	TLS TLSConfig `mapstructure:"tls"`

	// Security contains security hardening configuration
	Security SecurityConfig `mapstructure:"security"`

	// MaxMessageSize is the maximum size of a gRPC message
	MaxMessageSize int `mapstructure:"max_message_size"`

	// MaxConcurrentStreams is the maximum number of concurrent gRPC streams
	MaxConcurrentStreams uint32 `mapstructure:"max_concurrent_streams"`

	// KeepAlive contains gRPC keep-alive configuration
	KeepAlive KeepAliveConfig `mapstructure:"keep_alive"`

	// YANG contains YANG parser configuration
	YANG YANGConfig `mapstructure:"yang"`

	// Legacy fields for backward compatibility (deprecated)
	TLSEnabled       bool          `mapstructure:"tls_enabled"`
	TLSCertFile      string        `mapstructure:"tls_cert_file"`
	TLSKeyFile       string        `mapstructure:"tls_key_file"`
	TLSClientCAFile  string        `mapstructure:"tls_client_ca_file"`
	KeepAliveTimeout time.Duration `mapstructure:"keep_alive_timeout"`
}

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.ListenAddress == "" {
		return fmt.Errorf("listen_address cannot be empty")
	}

	// Validate TLS configuration (new format)
	if cfg.TLS.Enabled {
		if cfg.TLS.CertFile == "" {
			return fmt.Errorf("tls.cert_file is required when TLS is enabled")
		}
		if cfg.TLS.KeyFile == "" {
			return fmt.Errorf("tls.key_file is required when TLS is enabled")
		}

		// Validate TLS version settings
		if err := cfg.validateTLSVersions(); err != nil {
			return fmt.Errorf("tls version validation failed: %w", err)
		}

		// Validate client auth type
		if err := cfg.validateClientAuthType(); err != nil {
			return fmt.Errorf("tls client auth validation failed: %w", err)
		}
	}

	// Legacy validation for backward compatibility
	if cfg.TLSEnabled {
		if cfg.TLSCertFile == "" {
			return fmt.Errorf("tls_cert_file is required when TLS is enabled")
		}
		if cfg.TLSKeyFile == "" {
			return fmt.Errorf("tls_key_file is required when TLS is enabled")
		}
	}

	// Validate security configuration
	if err := cfg.validateSecurityConfig(); err != nil {
		return fmt.Errorf("security config validation failed: %w", err)
	}

	// Validate performance settings
	if cfg.MaxMessageSize < 0 {
		return fmt.Errorf("max_message_size must be non-negative")
	}
	if cfg.MaxConcurrentStreams == 0 {
		return fmt.Errorf("max_concurrent_streams must be greater than 0")
	}

	// Validate YANG settings
	if cfg.YANG.MaxModules < 0 {
		return fmt.Errorf("yang.max_modules must be non-negative")
	}

	return nil
}

// MigrateLegacyConfig migrates legacy configuration to new format
func (cfg *Config) MigrateLegacyConfig() {
	// Migrate TLS settings
	if cfg.TLSEnabled && !cfg.TLS.Enabled {
		cfg.TLS.Enabled = cfg.TLSEnabled
		cfg.TLS.CertFile = cfg.TLSCertFile
		cfg.TLS.KeyFile = cfg.TLSKeyFile
		cfg.TLS.CAFile = cfg.TLSClientCAFile
	}

	// Migrate keep-alive settings
	if cfg.KeepAliveTimeout > 0 && cfg.KeepAlive.Time == 0 {
		cfg.KeepAlive.Time = cfg.KeepAliveTimeout
		cfg.KeepAlive.Timeout = 10 * time.Second // Default timeout
	}
}

// validateTLSVersions validates TLS version settings
func (cfg *Config) validateTLSVersions() error {
	validVersions := map[string]bool{
		"1.0": true, "1.1": true, "1.2": true, "1.3": true,
	}

	if cfg.TLS.MinVersion != "" && !validVersions[cfg.TLS.MinVersion] {
		return fmt.Errorf("invalid tls.min_version: %s", cfg.TLS.MinVersion)
	}

	if cfg.TLS.MaxVersion != "" && !validVersions[cfg.TLS.MaxVersion] {
		return fmt.Errorf("invalid tls.max_version: %s", cfg.TLS.MaxVersion)
	}

	// Ensure min version is not greater than max version
	if cfg.TLS.MinVersion != "" && cfg.TLS.MaxVersion != "" {
		if cfg.TLS.MinVersion > cfg.TLS.MaxVersion {
			return fmt.Errorf("tls.min_version (%s) cannot be greater than tls.max_version (%s)",
				cfg.TLS.MinVersion, cfg.TLS.MaxVersion)
		}
	}

	return nil
}

// validateClientAuthType validates TLS client authentication type
func (cfg *Config) validateClientAuthType() error {
	if cfg.TLS.ClientAuthType == "" {
		return nil // Default is acceptable
	}

	validAuthTypes := map[string]bool{
		"NoClientCert":               true,
		"RequestClientCert":          true,
		"RequireAnyClientCert":       true,
		"VerifyClientCertIfGiven":    true,
		"RequireAndVerifyClientCert": true,
	}

	if !validAuthTypes[cfg.TLS.ClientAuthType] {
		return fmt.Errorf("invalid tls.client_auth_type: %s", cfg.TLS.ClientAuthType)
	}

	// If mTLS is required, ensure CA file is provided
	if (cfg.TLS.ClientAuthType == "RequireAnyClientCert" ||
		cfg.TLS.ClientAuthType == "VerifyClientCertIfGiven" ||
		cfg.TLS.ClientAuthType == "RequireAndVerifyClientCert") &&
		cfg.TLS.CAFile == "" {
		return fmt.Errorf("tls.ca_file is required for client certificate validation")
	}

	return nil
}

// validateSecurityConfig validates security configuration
func (cfg *Config) validateSecurityConfig() error {
	// Validate rate limiting configuration
	if cfg.Security.RateLimiting.Enabled {
		if cfg.Security.RateLimiting.RequestsPerSecond <= 0 {
			return fmt.Errorf("security.rate_limiting.requests_per_second must be positive")
		}
		if cfg.Security.RateLimiting.BurstSize < 0 {
			return fmt.Errorf("security.rate_limiting.burst_size must be non-negative")
		}
	}

	// Validate connection limits
	if cfg.Security.MaxConnections < 0 {
		return fmt.Errorf("security.max_connections must be non-negative")
	}

	// Validate connection timeout
	if cfg.Security.ConnectionTimeout < 0 {
		return fmt.Errorf("security.connection_timeout must be non-negative")
	}

	return nil
}
