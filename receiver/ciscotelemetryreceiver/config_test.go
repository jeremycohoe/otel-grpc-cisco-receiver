package ciscotelemetryreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_default_config",
			config: &Config{
				ListenAddress: "0.0.0.0:57500",
				TLS: TLSConfig{
					Enabled: false,
				},
				MaxMessageSize:       4 * 1024 * 1024,
				MaxConcurrentStreams: 100,
				YANG: YANGConfig{
					EnableRFCParser: true,
					CacheModules:    true,
					MaxModules:      1000,
				},
			},
			wantErr: false,
		},
		{
			name: "empty_listen_address",
			config: &Config{
				ListenAddress: "",
			},
			wantErr: true,
			errMsg:  "listen_address cannot be empty",
		},
		{
			name: "tls_enabled_without_cert",
			config: &Config{
				ListenAddress: "0.0.0.0:57500",
				TLS: TLSConfig{
					Enabled:  true,
					CertFile: "",
					KeyFile:  "key.pem",
				},
				MaxConcurrentStreams: 100,
			},
			wantErr: true,
			errMsg:  "tls.cert_file is required when TLS is enabled",
		},
		{
			name: "tls_enabled_without_key",
			config: &Config{
				ListenAddress: "0.0.0.0:57500",
				TLS: TLSConfig{
					Enabled:  true,
					CertFile: "cert.pem",
					KeyFile:  "",
				},
				MaxConcurrentStreams: 100,
			},
			wantErr: true,
			errMsg:  "tls.key_file is required when TLS is enabled",
		},
		{
			name: "legacy_tls_enabled_without_cert",
			config: &Config{
				ListenAddress:        "0.0.0.0:57500",
				TLSEnabled:           true,
				TLSCertFile:          "",
				TLSKeyFile:           "key.pem",
				MaxConcurrentStreams: 100,
			},
			wantErr: true,
			errMsg:  "tls_cert_file is required when TLS is enabled",
		},
		{
			name: "negative_max_message_size",
			config: &Config{
				ListenAddress:        "0.0.0.0:57500",
				MaxMessageSize:       -1,
				MaxConcurrentStreams: 100,
			},
			wantErr: true,
			errMsg:  "max_message_size must be non-negative",
		},
		{
			name: "zero_max_concurrent_streams",
			config: &Config{
				ListenAddress:        "0.0.0.0:57500",
				MaxConcurrentStreams: 0,
			},
			wantErr: true,
			errMsg:  "max_concurrent_streams must be greater than 0",
		},
		{
			name: "negative_max_modules",
			config: &Config{
				ListenAddress: "0.0.0.0:57500",
				YANG: YANGConfig{
					MaxModules: -1,
				},
				MaxConcurrentStreams: 100,
			},
			wantErr: true,
			errMsg:  "yang.max_modules must be non-negative",
		},
		{
			name: "valid_tls_config",
			config: &Config{
				ListenAddress: "0.0.0.0:57500",
				TLS: TLSConfig{
					Enabled:            true,
					CertFile:           "cert.pem",
					KeyFile:            "key.pem",
					CAFile:             "ca.pem",
					InsecureSkipVerify: false,
				},
				MaxMessageSize:       8 * 1024 * 1024,
				MaxConcurrentStreams: 200,
				KeepAlive: KeepAliveConfig{
					Time:    60 * time.Second,
					Timeout: 15 * time.Second,
				},
				YANG: YANGConfig{
					EnableRFCParser: true,
					CacheModules:    true,
					MaxModules:      2000,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_MigrateLegacyConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected *Config
	}{
		{
			name: "migrate_tls_settings",
			config: &Config{
				TLSEnabled:      true,
				TLSCertFile:     "cert.pem",
				TLSKeyFile:      "key.pem",
				TLSClientCAFile: "ca.pem",
			},
			expected: &Config{
				TLSEnabled:      true,
				TLSCertFile:     "cert.pem",
				TLSKeyFile:      "key.pem",
				TLSClientCAFile: "ca.pem",
				TLS: TLSConfig{
					Enabled:  true,
					CertFile: "cert.pem",
					KeyFile:  "key.pem",
					CAFile:   "ca.pem",
				},
			},
		},
		{
			name: "migrate_keepalive_settings",
			config: &Config{
				KeepAliveTimeout: 45 * time.Second,
			},
			expected: &Config{
				KeepAliveTimeout: 45 * time.Second,
				KeepAlive: KeepAliveConfig{
					Time:    45 * time.Second,
					Timeout: 10 * time.Second,
				},
			},
		},
		{
			name: "no_migration_needed",
			config: &Config{
				TLS: TLSConfig{
					Enabled:  true,
					CertFile: "cert.pem",
					KeyFile:  "key.pem",
				},
				KeepAlive: KeepAliveConfig{
					Time:    30 * time.Second,
					Timeout: 10 * time.Second,
				},
			},
			expected: &Config{
				TLS: TLSConfig{
					Enabled:  true,
					CertFile: "cert.pem",
					KeyFile:  "key.pem",
				},
				KeepAlive: KeepAliveConfig{
					Time:    30 * time.Second,
					Timeout: 10 * time.Second,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.MigrateLegacyConfig()
			assert.Equal(t, tt.expected.TLS, tt.config.TLS)
			assert.Equal(t, tt.expected.KeepAlive, tt.config.KeepAlive)
		})
	}
}

func TestTLSConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		tlsConfig TLSConfig
		valid     bool
	}{
		{
			name: "disabled_tls",
			tlsConfig: TLSConfig{
				Enabled: false,
			},
			valid: true,
		},
		{
			name: "enabled_tls_with_files",
			tlsConfig: TLSConfig{
				Enabled:  true,
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
			},
			valid: true,
		},
		{
			name: "mtls_config",
			tlsConfig: TLSConfig{
				Enabled:            true,
				CertFile:           "cert.pem",
				KeyFile:            "key.pem",
				CAFile:             "ca.pem",
				InsecureSkipVerify: false,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				ListenAddress:        "0.0.0.0:57500",
				TLS:                  tt.tlsConfig,
				MaxConcurrentStreams: 100,
			}
			err := config.Validate()
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestYANGConfig_Validation(t *testing.T) {
	tests := []struct {
		name       string
		yangConfig YANGConfig
		valid      bool
	}{
		{
			name: "default_yang_config",
			yangConfig: YANGConfig{
				EnableRFCParser: true,
				CacheModules:    true,
				MaxModules:      1000,
			},
			valid: true,
		},
		{
			name: "rfc_parser_disabled",
			yangConfig: YANGConfig{
				EnableRFCParser: false,
				CacheModules:    false,
				MaxModules:      0,
			},
			valid: true,
		},
		{
			name: "high_capacity_config",
			yangConfig: YANGConfig{
				EnableRFCParser: true,
				CacheModules:    true,
				MaxModules:      10000,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				ListenAddress:        "0.0.0.0:57500",
				YANG:                 tt.yangConfig,
				MaxConcurrentStreams: 100,
			}
			err := config.Validate()
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
