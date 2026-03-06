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
			name:    "valid_default_config",
			config:  createValidTestConfig(),
			wantErr: false,
		},
		{
			name: "empty_listen_address",
			config: &Config{
				ListenAddress:        "",
				MaxConcurrentStreams: 100,
			},
			wantErr: true,
			errMsg:  "listen_address must not be empty",
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
			name: "negative_max_recv_msg_size",
			config: &Config{
				ListenAddress:        "0.0.0.0:57500",
				MaxRecvMsgSizeMiB:    -1,
				MaxConcurrentStreams: 100,
			},
			wantErr: true,
			errMsg:  "max_recv_msg_size_mib must be non-negative",
		},
		{
			name: "negative_max_modules",
			config: &Config{
				ListenAddress:        "0.0.0.0:57500",
				MaxConcurrentStreams: 100,
				YANG:                 YANGConfig{MaxModules: -1},
			},
			wantErr: true,
			errMsg:  "yang.max_modules must be non-negative",
		},
		{
			name: "valid_full_config",
			config: &Config{
				ListenAddress:        "0.0.0.0:57500",
				MaxRecvMsgSizeMiB:    8,
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

func TestYANGConfig_Validation(t *testing.T) {
	tests := []struct {
		name  string
		yang  YANGConfig
		valid bool
	}{
		{
			name:  "defaults",
			yang:  YANGConfig{EnableRFCParser: true, CacheModules: true, MaxModules: 1000},
			valid: true,
		},
		{
			name:  "everything_off",
			yang:  YANGConfig{EnableRFCParser: false, CacheModules: false, MaxModules: 0},
			valid: true,
		},
		{
			name:  "high_capacity",
			yang:  YANGConfig{EnableRFCParser: true, CacheModules: true, MaxModules: 10000},
			valid: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ListenAddress:        "0.0.0.0:57500",
				MaxConcurrentStreams: 100,
				YANG:                 tt.yang,
			}
			err := cfg.Validate()
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
