package ciscotelemetryreceiver

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type ciscoTelemetryReceiver struct {
	config           *Config
	settings         receiver.Settings
	consumer         consumer.Metrics
	server           *grpc.Server
	listener         net.Listener
	wg               sync.WaitGroup
	telemetryBuilder *telemetryBuilder
	securityManager  *SecurityManager
}

func newCiscoTelemetryReceiver(
	config *Config,
	settings receiver.Settings,
	consumer consumer.Metrics,
) (*ciscoTelemetryReceiver, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Ensure we have a logger
	if settings.Logger == nil {
		settings.Logger = zap.NewNop()
	}

	// Migrate legacy config if needed
	config.MigrateLegacyConfig()

	// Initialize telemetry builder for internal observability
	telemetryBuilder, err := newTelemetryBuilder(settings.Logger, settings.MeterProvider.Meter("cisco_telemetry_receiver"))
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry builder: %w", err)
	}

	// Initialize security manager
	securityManager := NewSecurityManager(&config.Security, &config.TLS, settings.Logger)

	return &ciscoTelemetryReceiver{
		config:           config,
		settings:         settings,
		consumer:         consumer,
		telemetryBuilder: telemetryBuilder,
		securityManager:  securityManager,
	}, nil
}

func (ctr *ciscoTelemetryReceiver) Start(ctx context.Context, host component.Host) error {
	listener, err := net.Listen("tcp", ctr.config.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", ctr.config.ListenAddress, err)
	}
	ctr.listener = listener

	var opts []grpc.ServerOption

	// Configure TLS using security manager
	if ctr.config.TLS.Enabled || ctr.config.TLSEnabled {
		tlsConfig, err := ctr.securityManager.CreateTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to create TLS configuration: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
		}
	}

	// Add security interceptor for rate limiting and IP filtering
	securityInterceptor := ctr.securityManager.CreateSecurityInterceptor()
	opts = append(opts, grpc.UnaryInterceptor(securityInterceptor))

	// Configure keep-alive (new format first, then legacy)
	if ctr.config.KeepAlive.Time > 0 || ctr.config.KeepAliveTimeout > 0 {
		kaep := keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}

		keepAliveTime := ctr.config.KeepAlive.Time
		keepAliveTimeout := ctr.config.KeepAlive.Timeout

		// Fall back to legacy config if new config is empty
		if keepAliveTime == 0 && ctr.config.KeepAliveTimeout > 0 {
			keepAliveTime = ctr.config.KeepAliveTimeout
			keepAliveTimeout = 10 * time.Second
		}

		kasp := keepalive.ServerParameters{
			Time:    keepAliveTime,
			Timeout: keepAliveTimeout,
		}
		opts = append(opts, grpc.KeepaliveEnforcementPolicy(kaep), grpc.KeepaliveParams(kasp))
	}

	// Configure max message size and concurrent streams
	if ctr.config.MaxMessageSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(ctr.config.MaxMessageSize))
	}
	if ctr.config.MaxConcurrentStreams > 0 {
		opts = append(opts, grpc.MaxConcurrentStreams(ctr.config.MaxConcurrentStreams))
	}

	ctr.server = grpc.NewServer(opts...)

	// Initialize YANG parser with builtin modules
	yangParser := NewYANGParser()
	yangParser.LoadBuiltinModules()

	// Initialize RFC 6020/7950 compliant YANG parser
	rfcYangParser := NewRFC6020Parser()

	// Register the gRPC service for Cisco telemetry
	service := &grpcService{
		receiver:      ctr,
		yangParser:    yangParser,
		rfcYangParser: rfcYangParser,
	}
	pb.RegisterGRPCMdtDialoutServer(ctr.server, service)

	ctr.wg.Add(1)
	go func() {
		defer ctr.wg.Done()
		if err := ctr.server.Serve(listener); err != nil {
			ctr.settings.Logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	ctr.settings.Logger.Info("Cisco telemetry receiver started",
		zap.String("listen_address", ctr.config.ListenAddress),
		zap.Bool("tls_enabled", ctr.config.TLSEnabled))

	return nil
}

func (ctr *ciscoTelemetryReceiver) Shutdown(ctx context.Context) error {
	if ctr.server != nil {
		ctr.server.GracefulStop()
	}
	if ctr.listener != nil {
		ctr.listener.Close()
	}
	ctr.wg.Wait()

	// Clean up security manager
	if ctr.securityManager != nil {
		ctr.securityManager.Shutdown()
	}

	ctr.settings.Logger.Info("Cisco telemetry receiver stopped")
	return nil
}

// TODO: Implement the gRPC service methods for handling Cisco telemetry data
// This will need to process the kvGPB encoded data and convert to OTEL metrics
