package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jcohoe/otel-grpc-cisco-receiver/receiver/ciscotelemetryreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// DebugConsumer prints all received metrics to console
type DebugConsumer struct{}

func (d *DebugConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (d *DebugConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	fmt.Printf("\n=== TELEMETRY DATA RECEIVED ===\n")
	fmt.Printf("Timestamp: %s\n", time.Now().Format("15:04:05"))

	resourceMetrics := md.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		rm := resourceMetrics.At(i)
		scopeMetrics := rm.ScopeMetrics()

		for j := 0; j < scopeMetrics.Len(); j++ {
			sm := scopeMetrics.At(j)
			metrics := sm.Metrics()

			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				fmt.Printf("Metric: %s\n", metric.Name())

				// Print metric attributes and values
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					gauge := metric.Gauge()
					dataPoints := gauge.DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						fmt.Printf("  Value: %v\n", dp.DoubleValue())

						// Print attributes
						attrs := dp.Attributes()
						attrs.Range(func(k string, v pcommon.Value) bool {
							fmt.Printf("  %s: %s\n", k, v.AsString())
							return true
						})
					}
				}
				fmt.Println()
			}
		}
	}

	return nil
}

func main() {
	fmt.Println("Cisco Telemetry OTEL Receiver - Development Test")

	// Create a receiver factory
	factory := ciscotelemetryreceiver.NewFactory()

	// Create default config
	config := factory.CreateDefaultConfig()

	// Create a debug consumer that prints telemetry data
	debugConsumer := &DebugConsumer{}

	// Create receiver settings
	componentType := component.MustNewType("cisco_telemetry")
	settings := receiver.Settings{
		ID: component.NewIDWithName(componentType, "test"),
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}

	// Create the receiver
	rec, err := factory.CreateMetrics(
		context.Background(),
		settings,
		config,
		debugConsumer,
	)
	if err != nil {
		log.Fatalf("Failed to create receiver: %v", err)
	}

	fmt.Println("Receiver created successfully!")
	fmt.Printf("Config: %+v\n", config)

	// Start the receiver
	ctx := context.Background()
	err = rec.Start(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to start receiver: %v", err)
	}

	fmt.Println("Receiver started! Ready to receive Cisco telemetry...")
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down receiver...")
	err = rec.Shutdown(ctx)
	if err != nil {
		log.Printf("Error shutting down receiver: %v", err)
	}

	fmt.Println("Receiver stopped.")
}
