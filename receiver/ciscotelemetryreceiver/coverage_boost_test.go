package ciscotelemetryreceiver

import (
	"context"
	"testing"
	"time"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TestReceiverStart_ErrorScenarios tests Start method error scenarios to improve coverage
func TestReceiverStart_ErrorScenarios(t *testing.T) {
	withQuickTimeout(t, func(t *testing.T) {
		// Test Start with TLS enabled but missing cert files
		config := createValidTestConfig()
		config.TLS.Enabled = true
		config.TLS.CertFile = "nonexistent.crt"
		config.TLS.KeyFile = "nonexistent.key"

		consumer := &consumertest.MetricsSink{}
		settings := createTestSettings()

		receiver, err := newCiscoTelemetryReceiver(config, settings, consumer)
		require.NoError(t, err)

		ctx := context.Background()

		// Start should fail with missing cert files
		err = receiver.Start(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load TLS certificate")

		// Test Start with invalid listen address
		config2 := createValidTestConfig()
		config2.ListenAddress = "invalid:address:format"

		receiver2, err := newCiscoTelemetryReceiver(config2, settings, consumer)
		require.NoError(t, err)

		err = receiver2.Start(ctx, nil)
		assert.Error(t, err)
	})
}

// TestProcessGPBTableData tests the processGPBTableData method (0% coverage)
func TestProcessGPBTableData(t *testing.T) {
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

		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()

		// Create test GPB table data
		telemetry := &pb.Telemetry{
			EncodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces",
			DataGpb: &pb.TelemetryGPBTable{
				Row: []*pb.TelemetryRowGPB{
					{
						Timestamp: uint64(1234567890),
						Keys:      []byte("test-key-data"),
						Content:   []byte("test-content-data"),
					},
				},
			},
		}

		// Test processGPBTableData
		service.processGPBTableData(scopeMetrics, telemetry)

		// Should process without error (even if it can't fully parse the GPB data)
		// The method should at least attempt to process and not crash
	})
}

// TestGrpcService_EdgeCases tests edge cases to improve method coverage
func TestGrpcService_EdgeCases(t *testing.T) {
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

		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
		timestamp := pcommon.NewTimestampFromTime(time.Now())

		// Test processField with nested fields to improve coverage
		nestedField := &pb.TelemetryField{
			Name: "nested_container",
			Fields: []*pb.TelemetryField{
				{
					Name: "sub_field_1",
					ValueByType: &pb.TelemetryField_Uint64Value{
						Uint64Value: 100,
					},
				},
				{
					Name: "sub_field_2",
					ValueByType: &pb.TelemetryField_StringValue{
						StringValue: "nested_value",
					},
				},
			},
		}

		service.processField(scopeMetrics, nestedField, "/base/path", "/prefix", timestamp)
		assert.True(t, scopeMetrics.Metrics().Len() > 0)

		// Test processField with boolean value to increase coverage
		boolField := &pb.TelemetryField{
			Name: "bool_field",
			ValueByType: &pb.TelemetryField_BoolValue{
				BoolValue: true,
			},
		}

		service.processField(scopeMetrics, boolField, "/base/path", "/prefix", timestamp)

		// Test processField with sint64 value
		sintField := &pb.TelemetryField{
			Name: "sint_field",
			ValueByType: &pb.TelemetryField_Sint64Value{
				Sint64Value: -42,
			},
		}

		service.processField(scopeMetrics, sintField, "/base/path", "/prefix", timestamp)

		// Test extractFieldName with different path formats to improve coverage
		assert.Equal(t, "field", service.extractFieldName("path.to.field"))
		assert.Equal(t, "simple", service.extractFieldName("simple"))
		assert.Equal(t, "", service.extractFieldName(""))
		assert.Equal(t, "field", service.extractFieldName("field_info")) // Remove _info suffix

		// Test with dots in different positions
		assert.Equal(t, "last", service.extractFieldName("first.second.third.last"))
	})
}
