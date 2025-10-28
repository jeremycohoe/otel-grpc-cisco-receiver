package ciscotelemetryreceiver

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSecurityTLSConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid TLS config",
			config: &Config{
				ListenAddress: "localhost:0",
				TLS: TLSConfig{
					Enabled:        true,
					CertFile:       "test.crt",
					KeyFile:        "test.key",
					ClientAuthType: "NoClientCert",
					MinVersion:     "1.2",
					MaxVersion:     "1.3",
				},
				MaxConcurrentStreams: 100,
			},
			expectError: false,
		},
		{
			name: "Invalid TLS version - min greater than max",
			config: &Config{
				ListenAddress: "localhost:0",
				TLS: TLSConfig{
					Enabled:    true,
					CertFile:   "test.crt",
					KeyFile:    "test.key",
					MinVersion: "1.3",
					MaxVersion: "1.2",
				},
				MaxConcurrentStreams: 100,
			},
			expectError: true,
			errorMsg:    "tls.min_version (1.3) cannot be greater than tls.max_version (1.2)",
		},
		{
			name: "Invalid client auth type",
			config: &Config{
				ListenAddress: "localhost:0",
				TLS: TLSConfig{
					Enabled:        true,
					CertFile:       "test.crt",
					KeyFile:        "test.key",
					ClientAuthType: "InvalidAuthType",
				},
				MaxConcurrentStreams: 100,
			},
			expectError: true,
			errorMsg:    "invalid tls.client_auth_type: InvalidAuthType",
		},
		{
			name: "Missing CA file for mTLS",
			config: &Config{
				ListenAddress: "localhost:0",
				TLS: TLSConfig{
					Enabled:        true,
					CertFile:       "test.crt",
					KeyFile:        "test.key",
					ClientAuthType: "RequireAndVerifyClientCert",
					// CAFile is missing
				},
				MaxConcurrentStreams: 100,
			},
			expectError: true,
			errorMsg:    "tls.ca_file is required for client certificate validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSecurityConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid security config",
			config: &Config{
				ListenAddress: "localhost:0",
				Security: SecurityConfig{
					RateLimiting: RateLimitingConfig{
						Enabled:           true,
						RequestsPerSecond: 100.0,
						BurstSize:         10,
					},
					MaxConnections:    1000,
					ConnectionTimeout: 30 * time.Second,
				},
				MaxConcurrentStreams: 100,
			},
			expectError: false,
		},
		{
			name: "Invalid rate limiting - negative requests per second",
			config: &Config{
				ListenAddress: "localhost:0",
				Security: SecurityConfig{
					RateLimiting: RateLimitingConfig{
						Enabled:           true,
						RequestsPerSecond: -1.0,
						BurstSize:         10,
					},
				},
				MaxConcurrentStreams: 100,
			},
			expectError: true,
			errorMsg:    "requests_per_second must be positive",
		},
		{
			name: "Invalid max connections",
			config: &Config{
				ListenAddress: "localhost:0",
				Security: SecurityConfig{
					MaxConnections: -1,
				},
				MaxConcurrentStreams: 100,
			},
			expectError: true,
			errorMsg:    "max_connections must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSecurityManager_TLSConfig(t *testing.T) {
	// Create temporary certificate files
	certFile, keyFile, caFile := createTestCertificates(t)
	defer os.Remove(certFile)
	defer os.Remove(keyFile)
	defer os.Remove(caFile)

	tests := []struct {
		name        string
		tlsConfig   *TLSConfig
		expectError bool
		validate    func(*testing.T, *tls.Config)
	}{
		{
			name: "Basic TLS config",
			tlsConfig: &TLSConfig{
				Enabled:  true,
				CertFile: certFile,
				KeyFile:  keyFile,
			},
			expectError: false,
			validate: func(t *testing.T, tlsConf *tls.Config) {
				assert.Len(t, tlsConf.Certificates, 1)
				assert.Equal(t, uint16(tls.VersionTLS12), tlsConf.MinVersion)
				assert.Equal(t, uint16(tls.VersionTLS13), tlsConf.MaxVersion)
			},
		},
		{
			name: "mTLS config",
			tlsConfig: &TLSConfig{
				Enabled:        true,
				CertFile:       certFile,
				KeyFile:        keyFile,
				CAFile:         caFile,
				ClientAuthType: "RequireAndVerifyClientCert",
			},
			expectError: false,
			validate: func(t *testing.T, tlsConf *tls.Config) {
				assert.Equal(t, tls.RequireAndVerifyClientCert, tlsConf.ClientAuth)
				assert.NotNil(t, tlsConf.ClientCAs)
			},
		},
		{
			name: "Custom TLS versions",
			tlsConfig: &TLSConfig{
				Enabled:    true,
				CertFile:   certFile,
				KeyFile:    keyFile,
				MinVersion: "1.3",
				MaxVersion: "1.3",
			},
			expectError: false,
			validate: func(t *testing.T, tlsConf *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS13), tlsConf.MinVersion)
				assert.Equal(t, uint16(tls.VersionTLS13), tlsConf.MaxVersion)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			securityConfig := &SecurityConfig{}
			sm := NewSecurityManager(securityConfig, tt.tlsConfig, zap.NewNop())

			tlsConf, err := sm.CreateTLSConfig()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tlsConf != nil && tt.validate != nil {
					tt.validate(t, tlsConf)
				}
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(1.0, 2, time.Minute) // 1 request per second, burst of 2
	defer rl.Stop()

	// First request should be allowed (from burst)
	assert.True(t, rl.Allow("192.168.1.1"))

	// Second request should be allowed (from burst)
	assert.True(t, rl.Allow("192.168.1.1"))

	// Third request should be denied (rate limited - burst exhausted)
	assert.False(t, rl.Allow("192.168.1.1"))

	// Different IP should be allowed (has its own bucket)
	assert.True(t, rl.Allow("192.168.1.2"))
}

func TestSecurityManager_IPAllowlist(t *testing.T) {
	securityConfig := &SecurityConfig{
		AllowedClients: []string{"192.168.1.1", "10.0.0.0/8"},
	}
	tlsConfig := &TLSConfig{}
	sm := NewSecurityManager(securityConfig, tlsConfig, zap.NewNop())

	tests := []struct {
		clientIP string
		allowed  bool
	}{
		{"192.168.1.1", true},    // Exact match
		{"192.168.1.2", false},   // Not in list
		{"10.0.0.1", true},       // In CIDR range
		{"10.255.255.255", true}, // In CIDR range
		{"11.0.0.1", false},      // Not in CIDR range
	}

	for _, tt := range tests {
		t.Run(tt.clientIP, func(t *testing.T) {
			allowed := sm.isIPAllowed(tt.clientIP)
			assert.Equal(t, tt.allowed, allowed)
		})
	}
}

func TestSecurityManager_ClientAuthTypes(t *testing.T) {
	sm := &SecurityManager{}

	tests := []struct {
		authType string
		expected tls.ClientAuthType
		hasError bool
	}{
		{"NoClientCert", tls.NoClientCert, false},
		{"RequestClientCert", tls.RequestClientCert, false},
		{"RequireAnyClientCert", tls.RequireAnyClientCert, false},
		{"VerifyClientCertIfGiven", tls.VerifyClientCertIfGiven, false},
		{"RequireAndVerifyClientCert", tls.RequireAndVerifyClientCert, false},
		{"InvalidAuthType", tls.NoClientCert, true},
	}

	for _, tt := range tests {
		t.Run(tt.authType, func(t *testing.T) {
			authType, err := sm.getClientAuthType(tt.authType)
			if tt.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, authType)
			}
		})
	}
}

// createTestCertificates creates temporary certificate files for testing
func createTestCertificates(t *testing.T) (certFile, keyFile, caFile string) {
	// Generate CA private key
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create CA certificate
	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test CA"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	// Generate server private key
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create server certificate
	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization:  []string{"Test Server"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, &caTemplate, &serverKey.PublicKey, caKey)
	require.NoError(t, err)

	// Write CA certificate file
	caFile = writeToTempFile(t, "ca.crt", pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertDER,
	}))

	// Write server certificate file
	certFile = writeToTempFile(t, "server.crt", pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertDER,
	}))

	// Write server key file
	keyFile = writeToTempFile(t, "server.key", pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverKey),
	}))

	return certFile, keyFile, caFile
}

func writeToTempFile(t *testing.T, pattern string, data []byte) string {
	tmpFile, err := ioutil.TempFile("", pattern)
	require.NoError(t, err)

	_, err = tmpFile.Write(data)
	require.NoError(t, err)

	err = tmpFile.Close()
	require.NoError(t, err)

	return tmpFile.Name()
}
