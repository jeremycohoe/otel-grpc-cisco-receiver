//go:build benchmark

package ciscotelemetryreceiver

import (
	"context"
	"testing"
	"time"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"google.golang.org/protobuf/proto"
)

// Benchmark tests for measuring performance requirements
func BenchmarkTelemetryProcessing(b *testing.B) {
	// Create test receiver
	cfg := createValidTestConfig()
	settings := createTestSettings()
	consumer := consumertest.NewNop()

	receiver, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
	if err != nil {
		b.Fatalf("Failed to create receiver: %v", err)
	}

	// Create sample telemetry data
	telemetryData := createSampleTelemetryData()
	data, _ := proto.Marshal(telemetryData)

	req := &pb.MdtDialoutArgs{
		ReqId: 12345,
		Data:  data,
	}

	service := &grpcService{
		receiver:      receiver.(*ciscoTelemetryReceiver),
		yangParser:    NewYANGParser(),
		rfcYangParser: NewRFC6020Parser(),
	}

	// Reset timer and run benchmark
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = service.processTelemetryData(req)
	}
}

func BenchmarkYANGParsingRFC(b *testing.B) {
	parser := NewRFC6020Parser()
	encodingPath := "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = parser.AnalyzeTelemetryPath(encodingPath)
	}
}

func BenchmarkTelemetryBuilder_RecordMessage(b *testing.B) {
	settings := createTestSettings()

	tb, err := newTelemetryBuilder(settings.Logger, settings.MeterProvider.Meter("test"))
	if err != nil {
		b.Fatalf("Failed to create telemetry builder: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tb.RecordMessageReceived(ctx, "test-node", "12345", 1024)
		tb.RecordMessageProcessed(ctx, "test-node", "12345", "Cisco-IOS-XE-interfaces-oper", 10*time.Millisecond)
	}
}

func BenchmarkConcurrentTelemetryProcessing(b *testing.B) {
	// Test concurrent processing capability
	cfg := createValidTestConfig()
	settings := createTestSettings()
	consumer := consumertest.NewNop()

	receiver, err := createMetricsReceiver(context.Background(), settings, cfg, consumer)
	if err != nil {
		b.Fatalf("Failed to create receiver: %v", err)
	}

	service := &grpcService{
		receiver:      receiver.(*ciscoTelemetryReceiver),
		yangParser:    NewYANGParser(),
		rfcYangParser: NewRFC6020Parser(),
	}

	// Create multiple sample requests
	requests := make([]*pb.MdtDialoutArgs, 10)
	for i := 0; i < 10; i++ {
		telemetryData := createSampleTelemetryData()
		data, _ := proto.Marshal(telemetryData)
		requests[i] = &pb.MdtDialoutArgs{
			ReqId: int64(i),
			Data:  data,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := requests[i%len(requests)]
			_ = service.processTelemetryData(req)
			i++
		}
	})
}

// createSampleTelemetryData creates realistic telemetry data for benchmarks
func createSampleTelemetryData() *pb.Telemetry {
	return &pb.Telemetry{
		NodeId: &pb.Telemetry_NodeIdStr{
			NodeIdStr: "router-1.example.com",
		},
		Subscription: &pb.Telemetry_SubscriptionIdStr{
			SubscriptionIdStr: "test-subscription",
		},
		EncodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces",
		DataGpbkv: []*pb.TelemetryField{
			{
				Name: "name",
				Fields: []*pb.TelemetryField{
					{
						Name:        "name",
						ValueByType: &pb.TelemetryField_StringValue{StringValue: "GigabitEthernet0/0/1"},
					},
				},
			},
			{
				Name: "statistics",
				Fields: []*pb.TelemetryField{
					{
						Name:        "in-octets",
						ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 123456789},
					},
					{
						Name:        "out-octets",
						ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 987654321},
					},
					{
						Name:        "in-pkts",
						ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 1000},
					},
					{
						Name:        "out-pkts",
						ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 2000},
					},
				},
			},
		},
	}
}
