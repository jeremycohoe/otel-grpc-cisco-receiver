package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jcohoe/otel-grpc-cisco-receiver/receiver/ciscotelemetryreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("Cisco Telemetry OTEL Receiver - Standalone Mode")

	// Create a receiver factory
	factory := ciscotelemetryreceiver.NewFactory()

	// Create configuration
	config := &ciscotelemetryreceiver.Config{
		ListenAddress:    "0.0.0.0:57500",
		TLSEnabled:       false,
		KeepAliveTimeout: 60000000000, // 60 seconds in nanoseconds
		MaxMessageSize:   4194304,     // 4MB
	}

	// Create a consumer that prints metrics (you can replace this with Splunk HEC, etc.)
	consumer := &MetricsPrinter{}

	// Create receiver settings
	componentType := component.MustNewType("cisco_telemetry")
	settings := receiver.Settings{
		ID: component.NewIDWithName(componentType, "standalone"),
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.Must(zap.NewProduction()),
		},
	}

	// Create the receiver
	rec, err := factory.CreateMetrics(
		context.Background(),
		settings,
		config,
		consumer,
	)
	if err != nil {
		log.Fatalf("Failed to create receiver: %v", err)
	}

	fmt.Printf("Starting Cisco Telemetry Receiver on %s\n", config.ListenAddress)
	fmt.Println("Configuration:")
	fmt.Printf("  - Listen Address: %s\n", config.ListenAddress)
	fmt.Printf("  - TLS Enabled: %v\n", config.TLSEnabled)
	fmt.Printf("  - Keep Alive: %v\n", config.KeepAliveTimeout)
	fmt.Printf("  - Max Message Size: %d bytes\n", config.MaxMessageSize)

	// Start the receiver
	ctx := context.Background()
	err = rec.Start(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to start receiver: %v", err)
	}

	fmt.Println("✅ Receiver started! Waiting for Cisco telemetry connections...")
	fmt.Println("📊 Metrics will be printed to console")
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n🛑 Shutting down receiver...")
	err = rec.Shutdown(ctx)
	if err != nil {
		log.Printf("Error shutting down receiver: %v", err)
	}

	fmt.Println("✅ Receiver stopped.")
}

// MetricsPrinter is a simple consumer that prints received metrics
type MetricsPrinter struct {
	consumertest.MetricsSink
}

func (mp *MetricsPrinter) ConsumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	// Call the embedded sink to store metrics
	err := mp.MetricsSink.ConsumeMetrics(ctx, metrics)
	if err != nil {
		return err
	}

	// Print metrics information
	fmt.Printf("\n📈 Received Metrics Batch (Resource Count: %d)\n", metrics.ResourceMetrics().Len())

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		resource := rm.Resource()

		// Print resource attributes
		fmt.Printf("🔧 Resource Attributes:\n")
		resource.Attributes().Range(func(k string, v pcommon.Value) bool {
			fmt.Printf("   %s: %s\n", k, v.AsString())
			return true
		})

		// Print scope metrics
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			fmt.Printf("📊 Scope: %s (%d metrics)\n", sm.Scope().Name(), sm.Metrics().Len())

			// Print each metric
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				fmt.Printf("   • %s: %s\n", metric.Name(), metric.Description())

				// Print data points based on metric type
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					gauge := metric.Gauge()
					for l := 0; l < gauge.DataPoints().Len(); l++ {
						dp := gauge.DataPoints().At(l)
						fmt.Printf("     Value: %.2f (Timestamp: %d)\n",
							dp.DoubleValue(), dp.Timestamp())
					}
				}
			}
		}
	}
	fmt.Printf("✅ Processed %d metrics\n", metrics.MetricCount())
	return nil
}
