package ciscotelemetryreceiver

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/configtls"
)

// KeepAliveConfig contains gRPC keep-alive configuration.
type KeepAliveConfig struct {
	// Time is the duration between server-side keep-alive pings.
	Time time.Duration `mapstructure:"time"`

	// Timeout is the duration the server waits for a keep-alive response.
	Timeout time.Duration `mapstructure:"timeout"`

	// EnforcementMinTime is the minimum interval clients must wait between pings.
	EnforcementMinTime time.Duration `mapstructure:"enforcement_min_time"`

	// EnforcementPermitNoStream allows keep-alive pings when there are no active streams.
	EnforcementPermitNoStream bool `mapstructure:"enforcement_permit_no_stream"`
}

// YANGConfig contains YANG parser configuration.
type YANGConfig struct {
	// EnableRFCParser enables the RFC 6020/7950 compliant YANG parser.
	EnableRFCParser bool `mapstructure:"enable_rfc_parser"`

	// CacheModules enables caching of discovered YANG modules.
	CacheModules bool `mapstructure:"cache_modules"`

	// MaxModules is the maximum number of YANG modules to cache.
	MaxModules int `mapstructure:"max_modules"`

	// ModelsDir is the path to a directory containing .yang files.
	// On first run (or when files change), the RFC parser reads all .yang files
	// and caches the extracted metadata to CacheFile for instant startup.
	ModelsDir string `mapstructure:"models_dir"`

	// CacheFile is the path to the serialized YANG module cache.
	// Auto-generated from ModelsDir contents. Defaults to <ModelsDir>/yang-cache.json.
	CacheFile string `mapstructure:"cache_file"`
}

// Config represents the receiver configuration within the collector config.yaml.
type Config struct {
	// ListenAddress is the host:port to bind for incoming gRPC connections.
	ListenAddress string `mapstructure:"listen_address"`

	// TLS configures TLS / mTLS for the gRPC server using the standard
	// OpenTelemetry configtls.ServerConfig. This handles certificate paths,
	// client CA, min/max TLS version, client_auth, reload_interval, etc.
	// When nil, the server runs without TLS (plaintext).
	TLS *configtls.ServerConfig `mapstructure:"tls,omitempty"`

	// MaxRecvMsgSizeMiB is the maximum gRPC receive message size in MiB.
	MaxRecvMsgSizeMiB int `mapstructure:"max_recv_msg_size_mib"`

	// MaxConcurrentStreams caps the number of concurrent gRPC streams.
	MaxConcurrentStreams uint32 `mapstructure:"max_concurrent_streams"`

	// KeepAlive contains gRPC keep-alive configuration.
	KeepAlive KeepAliveConfig `mapstructure:"keepalive"`

	// YANG contains YANG parser configuration.
	YANG YANGConfig `mapstructure:"yang"`
}

// Validate checks the receiver configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.ListenAddress == "" {
		return fmt.Errorf("listen_address must not be empty")
	}

	if cfg.MaxRecvMsgSizeMiB < 0 {
		return fmt.Errorf("max_recv_msg_size_mib must be non-negative")
	}

	if cfg.MaxConcurrentStreams == 0 {
		return fmt.Errorf("max_concurrent_streams must be greater than 0")
	}

	if cfg.YANG.MaxModules < 0 {
		return fmt.Errorf("yang.max_modules must be non-negative")
	}

	return nil
}
