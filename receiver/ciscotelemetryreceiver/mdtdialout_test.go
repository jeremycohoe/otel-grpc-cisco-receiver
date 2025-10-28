package ciscotelemetryreceiver

import (
	"context"
	"io"
	"testing"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// mockMdtStream implements a simple mock for testing MdtDialout method
type mockMdtStream struct {
	grpc.BidiStreamingServer[*pb.MdtDialoutArgs, *pb.MdtDialoutArgs]
	recvCount int
	sendCount int
	ctx       context.Context
}

func (m *mockMdtStream) Recv() (*pb.MdtDialoutArgs, error) {
	m.recvCount++

	if m.recvCount == 1 {
		// Return valid telemetry data on first call
		validTelemetry := &pb.Telemetry{
			NodeId:       &pb.Telemetry_NodeIdStr{NodeIdStr: "test-node"},
			EncodingPath: "simple:test",
			MsgTimestamp: 1234567890,
		}
		validData, _ := proto.Marshal(validTelemetry)

		return &pb.MdtDialoutArgs{
			ReqId: 12345,
			Data:  validData,
		}, nil
	}

	if m.recvCount == 2 {
		// Return empty data on second call to test error handling
		return &pb.MdtDialoutArgs{
			ReqId: 12346,
			Data:  []byte{},
		}, nil
	}

	// Return EOF on third call to end the stream
	return nil, io.EOF
}

func (m *mockMdtStream) Send(response *pb.MdtDialoutArgs) error {
	m.sendCount++
	// Just track that Send was called
	return nil
}

func (m *mockMdtStream) Context() context.Context {
	return m.ctx
}

// TestMdtDialout_MockFlow tests the MdtDialout method with mock stream
func TestMdtDialout_MockFlow(t *testing.T) {
	withQuickTimeout(t, func(t *testing.T) {
		config := createValidTestConfig()
		consumer := &consumertest.MetricsSink{}
		settings := createTestSettings()

		receiver, err := newCiscoTelemetryReceiver(config, settings, consumer)
		require.NoError(t, err)

		yangParser := NewYANGParser()
		yangParser.LoadBuiltinModules()

		service := &grpcService{
			receiver:   receiver,
			yangParser: yangParser,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockStream := &mockMdtStream{
			ctx: ctx,
		}

		// Test MdtDialout method - should process messages and handle EOF gracefully
		err = service.MdtDialout(mockStream)
		assert.NoError(t, err) // Should return nil on EOF

		// Verify the mock received the expected calls
		assert.Equal(t, 3, mockStream.recvCount) // Should have called Recv 3 times
		assert.Equal(t, 2, mockStream.sendCount) // Should have sent 2 responses (1 success, 1 error)
	})
}
