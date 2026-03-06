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

	// Initialize telemetry builder for internal observability
	telemetryBuilder, err := newTelemetryBuilder(settings.Logger, settings.MeterProvider.Meter("cisco_telemetry_receiver"))
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry builder: %w", err)
	}

	return &ciscoTelemetryReceiver{
		config:           config,
		settings:         settings,
		consumer:         consumer,
		telemetryBuilder: telemetryBuilder,
	}, nil
}

func (ctr *ciscoTelemetryReceiver) Start(ctx context.Context, host component.Host) error {
	var opts []grpc.ServerOption

	// Configure TLS / mTLS via OTel configtls when provided.
	if ctr.config.TLS != nil {
		tlsCfg, err := ctr.config.TLS.LoadTLSConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to load TLS config: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	// Keep-alive settings.
	if ctr.config.KeepAlive.Time > 0 {
		kasp := keepalive.ServerParameters{
			Time:    ctr.config.KeepAlive.Time,
			Timeout: ctr.config.KeepAlive.Timeout,
		}
		minTime := ctr.config.KeepAlive.EnforcementMinTime
		if minTime == 0 {
			minTime = 30 * time.Second
		}
		kaep := keepalive.EnforcementPolicy{
			MinTime:             minTime,
			PermitWithoutStream: ctr.config.KeepAlive.EnforcementPermitNoStream,
		}
		opts = append(opts, grpc.KeepaliveParams(kasp), grpc.KeepaliveEnforcementPolicy(kaep))
	}

	// Max receive message size (config is in MiB, gRPC wants bytes).
	if ctr.config.MaxRecvMsgSizeMiB > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(ctr.config.MaxRecvMsgSizeMiB*1024*1024))
	}
	if ctr.config.MaxConcurrentStreams > 0 {
		opts = append(opts, grpc.MaxConcurrentStreams(ctr.config.MaxConcurrentStreams))
	}

	// Create listener.
	listener, err := net.Listen("tcp", ctr.config.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", ctr.config.ListenAddress, err)
	}
	ctr.listener = listener

	ctr.server = grpc.NewServer(opts...)

	// Initialize YANG parsers.
	yangParser := NewYANGParser()
	yangParser.LoadBuiltinModules()
	rfcYangParser := NewRFC6020ParserWithLogger(ctr.settings.Logger)

	// Register the Cisco MDT gRPC dial-out service.
	service := &grpcService{
		receiver:      ctr,
		yangParser:    yangParser,
		rfcYangParser: rfcYangParser,
	}
	pb.RegisterGRPCMdtDialoutServer(ctr.server, service)

	// Serve in background goroutine.
	ctr.wg.Add(1)
	go func() {
		defer ctr.wg.Done()
		if err := ctr.server.Serve(listener); err != nil {
			ctr.settings.Logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	tlsEnabled := ctr.config.TLS != nil
	ctr.settings.Logger.Info("Cisco telemetry receiver started",
		zap.String("listen_address", ctr.config.ListenAddress),
		zap.Bool("tls_enabled", tlsEnabled))

	return nil
}

func (ctr *ciscoTelemetryReceiver) Shutdown(ctx context.Context) error {
	if ctr.server != nil {
		// GracefulStop closes the listener internally, so we don't call
		// listener.Close() separately (avoids double-close).
		stopped := make(chan struct{})
		go func() {
			ctr.server.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-ctx.Done():
			// Context deadline exceeded — force stop.
			ctr.server.Stop()
		}
	}
	ctr.wg.Wait()

	ctr.settings.Logger.Info("Cisco telemetry receiver stopped")
	return nil
}
