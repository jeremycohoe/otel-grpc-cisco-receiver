//go:build integration

package ciscotelemetryreceiver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func TestCiscoTelemetryReceiverBasicIntegration(t *testing.T) {
	// Create a mock consumer to capture metrics
	consumer := consumertest.NewNop()

	// Create receiver configuration
	config := &Config{
		ListenAddress: "localhost:57400",
		TLSEnabled:    false,
	}

	// Create receiver factory
	typeStr := component.MustNewType(TypeStr)
	settings := receiver.Settings{
		ID: component.NewID(typeStr),
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
		BuildInfo: component.NewDefaultBuildInfo(),
	}

	// Create receiver using the factory method
	ctx := context.Background()
	rcvr, err := createMetricsReceiver(ctx, settings, config, consumer)
	if err != nil {
		t.Fatalf("Failed to create receiver: %v", err)
	}

	// Create a simple host implementation
	host := &testHost{}

	// Start receiver
	err = rcvr.Start(ctx, host)
	if err != nil {
		t.Fatalf("Failed to start receiver: %v", err)
	}

	// Give the receiver time to start
	time.Sleep(50 * time.Millisecond)

	// Send test telemetry data
	err = sendTestTelemetry("localhost:57400")
	if err != nil {
		t.Fatalf("Failed to send test data: %v", err)
	}

	// Wait for data processing
	time.Sleep(100 * time.Millisecond)

	// Shutdown receiver
	err = rcvr.Shutdown(ctx)
	if err != nil {
		t.Errorf("Failed to shutdown receiver: %v", err)
	}

	t.Log("Basic integration test completed successfully")
}

func TestMultipleConnectionsIntegration(t *testing.T) {
	consumer := consumertest.NewNop()

	config := &Config{
		ListenAddress: "localhost:57401",
		TLSEnabled:    false,
	}

	typeStr := component.MustNewType(TypeStr)
	settings := receiver.Settings{
		ID: component.NewID(typeStr),
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
		BuildInfo: component.NewDefaultBuildInfo(),
	}

	ctx := context.Background()
	rcvr, err := createMetricsReceiver(ctx, settings, config, consumer)
	if err != nil {
		t.Fatalf("Failed to create receiver: %v", err)
	}

	host := &testHost{}

	err = rcvr.Start(ctx, host)
	if err != nil {
		t.Fatalf("Failed to start receiver: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send data from multiple concurrent connections
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := sendTestTelemetryWithNodeID("localhost:57401", fmt.Sprintf("switch-%d", id))
			if err != nil {
				t.Errorf("Failed to send test data from connection %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	err = rcvr.Shutdown(ctx)
	if err != nil {
		t.Errorf("Failed to shutdown receiver: %v", err)
	}

	t.Log("Multiple connections integration test completed successfully")
}

// testHost implements component.Host interface for testing
type testHost struct{}

func (h *testHost) ReportFatalError(err error) {
	// No-op for testing
}

func (h *testHost) GetFactory(component.Kind, component.Type) component.Factory {
	return nil
}

func (h *testHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (h *testHost) GetExporters() map[component.Kind]map[component.ID]component.Component {
	return nil
}

// sendTestTelemetry sends mock Cisco telemetry data to the receiver
func sendTestTelemetry(endpoint string) error {
	return sendTestTelemetryWithNodeID(endpoint, "test-switch")
}

// sendTestTelemetryWithNodeID sends mock Cisco telemetry data with a specific node ID
func sendTestTelemetryWithNodeID(endpoint, nodeID string) error {
	// Connect to the receiver
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to receiver: %v", err)
	}
	defer conn.Close()

	client := pb.NewGRPCMdtDialoutClient(conn)

	// Create a bidirectional stream
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.MdtDialout(ctx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %v", err)
	}

	// Create test telemetry data
	telemetryData := &pb.Telemetry{
		NodeId:              &pb.Telemetry_NodeIdStr{NodeIdStr: nodeID},
		Subscription:        &pb.Telemetry_SubscriptionIdStr{SubscriptionIdStr: "test-sub"},
		EncodingPath:        "Cisco-IOS-XE-interfaces-oper:interfaces/interface",
		CollectionId:        1,
		CollectionStartTime: uint64(time.Now().UnixNano()),
		MsgTimestamp:        uint64(time.Now().UnixNano()),
		DataGpbkv: []*pb.TelemetryField{
			{
				Name: "interface",
				Fields: []*pb.TelemetryField{
					{
						Name:        "name",
						ValueByType: &pb.TelemetryField_StringValue{StringValue: "GigabitEthernet1/0/1"},
					},
					{
						Name: "statistics",
						Fields: []*pb.TelemetryField{
							{
								Name:        "rx-pkts",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 1000},
							},
							{
								Name:        "tx-pkts",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 1500},
							},
							{
								Name:        "rx-bytes",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 64000},
							},
							{
								Name:        "tx-bytes",
								ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 96000},
							},
						},
					},
				},
			},
		},
	}

	// Serialize telemetry data
	telemetryBytes, err := proto.Marshal(telemetryData)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry data: %v", err)
	}

	// Send the telemetry data
	args := &pb.MdtDialoutArgs{
		ReqId: 1,
		Data:  telemetryBytes,
	}

	err = stream.Send(args)
	if err != nil {
		return fmt.Errorf("failed to send telemetry data: %v", err)
	}

	// Close the send side of the stream
	err = stream.CloseSend()
	if err != nil {
		return fmt.Errorf("failed to close send stream: %v", err)
	}

	// Try to receive any response (optional for dial-out)
	_, err = stream.Recv()
	if err != nil && err.Error() != "EOF" {
		// EOF is expected since we closed the stream
		return fmt.Errorf("unexpected error receiving response: %v", err)
	}

	return nil
}

// TestReceiverConfiguration tests various configuration scenarios
func TestReceiverConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "Valid basic config",
			config: &Config{
				ListenAddress: "localhost:57500",
				TLSEnabled:    false,
			},
			expectError: false,
		},
		{
			name: "Empty address",
			config: &Config{
				ListenAddress: "",
				TLSEnabled:    false,
			},
			expectError: true,
		},
		{
			name: "TLS enabled without cert files",
			config: &Config{
				ListenAddress: "localhost:57500",
				TLSEnabled:    true,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
