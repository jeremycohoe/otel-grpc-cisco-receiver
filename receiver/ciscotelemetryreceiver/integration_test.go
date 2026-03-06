//go:build integration

package ciscotelemetryreceiver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// metricsCapture is a consumer.Metrics that stores everything it receives.
type metricsCapture struct {
	mu      sync.Mutex
	batches []pmetric.Metrics
}

func (m *metricsCapture) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *metricsCapture) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cloned := pmetric.NewMetrics()
	md.CopyTo(cloned)
	m.batches = append(m.batches, cloned)
	return nil
}

func (m *metricsCapture) all() []pmetric.Metrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]pmetric.Metrics, len(m.batches))
	copy(out, m.batches)
	return out
}

// testHost implements component.Host.
type testHost struct{}

func (h *testHost) ReportFatalError(error) {}

// --- helpers ----------------------------------------------------------------

func startReceiver(t *testing.T, addr string, cons consumer.Metrics) func() {
	t.Helper()
	cfg := createValidTestConfig()
	cfg.ListenAddress = addr
	settings := createTestSettings()

	rcv, err := newCiscoTelemetryReceiver(cfg, settings, cons)
	if err != nil {
		t.Fatalf("create receiver: %v", err)
	}
	if err := rcv.Start(context.Background(), &testHost{}); err != nil {
		t.Fatalf("start receiver: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // let gRPC bind
	return func() { rcv.Shutdown(context.Background()) }
}

func sendTelemetry(endpoint, nodeID string) error {
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewGRPCMdtDialoutClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.MdtDialout(ctx)
	if err != nil {
		return err
	}

	telemetryMsg := &pb.Telemetry{
		NodeId:       &pb.Telemetry_NodeIdStr{NodeIdStr: nodeID},
		Subscription: &pb.Telemetry_SubscriptionIdStr{SubscriptionIdStr: "sub1"},
		EncodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
		MsgTimestamp: uint64(time.Now().UnixMilli()),
		DataGpbkv: []*pb.TelemetryField{
			{
				Name: "interface",
				Fields: []*pb.TelemetryField{
					{Name: "name", ValueByType: &pb.TelemetryField_StringValue{StringValue: "GigabitEthernet1/0/1"}},
					{
						Name: "statistics",
						Fields: []*pb.TelemetryField{
							{Name: "rx-pkts", ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 1000}},
							{Name: "tx-pkts", ValueByType: &pb.TelemetryField_Uint64Value{Uint64Value: 1500}},
						},
					},
				},
			},
		},
	}

	data, err := proto.Marshal(telemetryMsg)
	if err != nil {
		return err
	}

	if err := stream.Send(&pb.MdtDialoutArgs{ReqId: 1, Data: data}); err != nil {
		return err
	}
	_ = stream.CloseSend()
	_, _ = stream.Recv() // consume ack or EOF
	return nil
}

// --- tests ------------------------------------------------------------------

func TestBasicIntegration(t *testing.T) {
	cap := &metricsCapture{}
	stop := startReceiver(t, "localhost:57410", cap)
	defer stop()

	if err := sendTelemetry("localhost:57410", "switch-1"); err != nil {
		t.Fatalf("send telemetry: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	if len(cap.all()) == 0 {
		t.Fatal("expected captured metrics, got none")
	}
	t.Logf("captured %d metric batches", len(cap.all()))
}

func TestMultipleConnections(t *testing.T) {
	cap := &metricsCapture{}
	stop := startReceiver(t, "localhost:57411", cap)
	defer stop()

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := sendTelemetry("localhost:57411", fmt.Sprintf("switch-%d", id)); err != nil {
				t.Errorf("connection %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	batches := cap.all()
	if len(batches) < 3 {
		t.Errorf("expected >=3 batches, got %d", len(batches))
	}
}
