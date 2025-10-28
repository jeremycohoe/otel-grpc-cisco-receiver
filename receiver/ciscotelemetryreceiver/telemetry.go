package ciscotelemetryreceiver

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// telemetryBuilder handles internal observability metrics for the receiver
type telemetryBuilder struct {
	logger *zap.Logger

	// Metrics for internal observability
	messagesReceived      metric.Int64Counter
	messagesProcessed     metric.Int64Counter
	messagesDropped       metric.Int64Counter
	bytesReceived         metric.Int64Counter
	connectionsActive     metric.Int64UpDownCounter
	yangModulesDiscovered metric.Int64UpDownCounter
	processingDuration    metric.Int64Histogram
	grpcErrors            metric.Int64Counter

	// Internal state tracking
	activeConnections int64
	discoveredModules map[string]bool
	modulesMutex      sync.RWMutex
}

// newTelemetryBuilder creates a new telemetry builder for internal metrics
func newTelemetryBuilder(logger *zap.Logger, meter metric.Meter) (*telemetryBuilder, error) {
	tb := &telemetryBuilder{
		logger:            logger,
		discoveredModules: make(map[string]bool),
	}

	var err error

	// Create internal metrics
	tb.messagesReceived, err = meter.Int64Counter(
		"cisco_telemetry_receiver_messages_received",
		metric.WithDescription("Number of telemetry messages received"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	tb.messagesProcessed, err = meter.Int64Counter(
		"cisco_telemetry_receiver_messages_processed",
		metric.WithDescription("Number of telemetry messages successfully processed"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	tb.messagesDropped, err = meter.Int64Counter(
		"cisco_telemetry_receiver_messages_dropped",
		metric.WithDescription("Number of telemetry messages dropped due to errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	tb.bytesReceived, err = meter.Int64Counter(
		"cisco_telemetry_receiver_bytes_received",
		metric.WithDescription("Total bytes received from telemetry connections"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	tb.connectionsActive, err = meter.Int64UpDownCounter(
		"cisco_telemetry_receiver_connections_active",
		metric.WithDescription("Number of active gRPC connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	tb.yangModulesDiscovered, err = meter.Int64UpDownCounter(
		"cisco_telemetry_receiver_yang_modules_discovered",
		metric.WithDescription("Number of unique YANG modules discovered"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	tb.processingDuration, err = meter.Int64Histogram(
		"cisco_telemetry_receiver_processing_duration",
		metric.WithDescription("Time spent processing telemetry messages"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1.0, 5.0, 10.0, 50.0, 100.0, 500.0, 1000.0),
	)
	if err != nil {
		return nil, err
	}

	tb.grpcErrors, err = meter.Int64Counter(
		"cisco_telemetry_receiver_grpc_errors",
		metric.WithDescription("Number of gRPC errors encountered"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	return tb, nil
}

// RecordMessageReceived records a received message
func (tb *telemetryBuilder) RecordMessageReceived(ctx context.Context, nodeID, subscriptionID string, bytes int64) {
	attrs := []attribute.KeyValue{
		attribute.String("node_id", nodeID),
		attribute.String("subscription_id", subscriptionID),
	}

	tb.messagesReceived.Add(ctx, 1, metric.WithAttributes(attrs...))
	tb.bytesReceived.Add(ctx, bytes, metric.WithAttributes(attrs...))
}

// RecordMessageProcessed records a successfully processed message
func (tb *telemetryBuilder) RecordMessageProcessed(ctx context.Context, nodeID, subscriptionID, yangModule string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("node_id", nodeID),
		attribute.String("subscription_id", subscriptionID),
		attribute.String("yang_module", yangModule),
	}

	tb.messagesProcessed.Add(ctx, 1, metric.WithAttributes(attrs...))
	tb.processingDuration.Record(ctx, duration.Milliseconds(), metric.WithAttributes(attrs...))

	// Track unique YANG modules
	tb.trackYANGModule(ctx, yangModule)
}

// RecordMessageDropped records a dropped message with error reason
func (tb *telemetryBuilder) RecordMessageDropped(ctx context.Context, nodeID, subscriptionID, reason string) {
	attrs := []attribute.KeyValue{
		attribute.String("node_id", nodeID),
		attribute.String("subscription_id", subscriptionID),
		attribute.String("reason", reason),
	}

	tb.messagesDropped.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordGRPCError records a gRPC error
func (tb *telemetryBuilder) RecordGRPCError(ctx context.Context, errorType, errorCode string) {
	attrs := []attribute.KeyValue{
		attribute.String("error_type", errorType),
		attribute.String("error_code", errorCode),
	}

	tb.grpcErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordConnectionOpened records a new gRPC connection
func (tb *telemetryBuilder) RecordConnectionOpened(ctx context.Context, clientAddr string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_addr", clientAddr),
	}

	tb.activeConnections++
	tb.connectionsActive.Add(ctx, 1, metric.WithAttributes(attrs...))

	tb.logger.Info("gRPC connection opened",
		zap.String("client_addr", clientAddr),
		zap.Int64("total_active", tb.activeConnections))
}

// RecordConnectionClosed records a closed gRPC connection
func (tb *telemetryBuilder) RecordConnectionClosed(ctx context.Context, clientAddr string) {
	attrs := []attribute.KeyValue{
		attribute.String("client_addr", clientAddr),
	}

	tb.activeConnections--
	tb.connectionsActive.Add(ctx, -1, metric.WithAttributes(attrs...))

	tb.logger.Info("gRPC connection closed",
		zap.String("client_addr", clientAddr),
		zap.Int64("total_active", tb.activeConnections))
}

// trackYANGModule tracks unique YANG modules discovered
func (tb *telemetryBuilder) trackYANGModule(ctx context.Context, yangModule string) {
	tb.modulesMutex.Lock()
	defer tb.modulesMutex.Unlock()

	if !tb.discoveredModules[yangModule] {
		tb.discoveredModules[yangModule] = true
		tb.yangModulesDiscovered.Add(ctx, 1, metric.WithAttributes(
			attribute.String("yang_module", yangModule),
		))

		tb.logger.Info("New YANG module discovered",
			zap.String("yang_module", yangModule),
			zap.Int("total_modules", len(tb.discoveredModules)))
	}
}

// GetActiveConnections returns the current number of active connections
func (tb *telemetryBuilder) GetActiveConnections() int64 {
	return tb.activeConnections
}

// GetDiscoveredModulesCount returns the number of unique YANG modules discovered
func (tb *telemetryBuilder) GetDiscoveredModulesCount() int {
	tb.modulesMutex.RLock()
	defer tb.modulesMutex.RUnlock()
	return len(tb.discoveredModules)
}

// GetDiscoveredModules returns a slice of discovered YANG module names
func (tb *telemetryBuilder) GetDiscoveredModules() []string {
	tb.modulesMutex.RLock()
	defer tb.modulesMutex.RUnlock()

	modules := make([]string, 0, len(tb.discoveredModules))
	for module := range tb.discoveredModules {
		modules = append(modules, module)
	}
	return modules
}
