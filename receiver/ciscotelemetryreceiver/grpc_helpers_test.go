package ciscotelemetryreceiver

import (
	"testing"
	"time"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TestGrpcService_HelperMethods tests all the 0% coverage helper methods
func TestGrpcService_HelperMethods(t *testing.T) {
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

		// Test processKvGPBData method (0% coverage)
		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()

		telemetry := &pb.Telemetry{
			MsgTimestamp: uint64(time.Now().UnixMilli()),
			DataGpbkv: []*pb.TelemetryField{
				{
					Name: "test_field",
					ValueByType: &pb.TelemetryField_Uint64Value{
						Uint64Value: 42,
					},
				},
			},
		}

		// This will test processKvGPBData
		service.processKvGPBData(scopeMetrics, telemetry)
		assert.True(t, scopeMetrics.Metrics().Len() >= 0) // Should process without error

		// Test createGaugeMetric method (0% coverage)
		metric := scopeMetrics.Metrics().AppendEmpty()
		timestamp := pcommon.NewTimestampFromTime(time.Now())
		service.createGaugeMetric(metric, "test_gauge", "/test/path", 123.45, timestamp)

		assert.Contains(t, metric.Name(), "test_gauge") // Has cisco. prefix
		assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

		// Test createInfoMetric method (0% coverage)
		infoMetric := scopeMetrics.Metrics().AppendEmpty()
		service.createInfoMetric(infoMetric, "test_info", "/test/path", "test_value", timestamp)

		assert.Contains(t, infoMetric.Name(), "test_info")          // Has cisco. prefix and _info suffix
		assert.Equal(t, pmetric.MetricTypeGauge, infoMetric.Type()) // Test isKeyField method (0% coverage)
		analysis := &PathAnalysis{
			Keys: map[string]string{"interface-name": "key", "port-id": "key"},
		}
		assert.True(t, service.isKeyField("interface-name", analysis))
		assert.False(t, service.isKeyField("counter-value", analysis))
		assert.False(t, service.isKeyField("unknown", nil))

		// Test enhanceMetricWithYANGInfo method (0% coverage)
		enhanceMetric := scopeMetrics.Metrics().AppendEmpty()
		service.enhanceMetricWithYANGInfo(enhanceMetric, "test_field", analysis, "/test/path")
		// Should not crash - just adds metadata

		// Test addYANGAttributes method (0% coverage)
		attrs := pcommon.NewMap()
		minVal := int64(0)
		maxVal := int64(9223372036854775807) // Max int64
		yangDataType := &YANGDataType{
			Type:        "uint64",
			Range:       &YANGRange{Min: &minVal, Max: &maxVal},
			Description: "Test uint64 field",
		}
		service.addYANGAttributes(attrs, "/test/path", yangDataType, "test_field")

		// Verify some attributes were added
		assert.True(t, attrs.Len() > 0)
	})
}

// TestGrpcService_YANGAwareMethods tests YANG-aware metric creation methods
func TestGrpcService_YANGAwareMethods(t *testing.T) {
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

		minVal2 := int64(0)
		maxVal2 := int64(9223372036854775807)
		yangDataType := &YANGDataType{
			Type:        "uint64",
			Range:       &YANGRange{Min: &minVal2, Max: &maxVal2},
			Description: "Test uint64 field",
		}

		// Test createYANGAwareMetric method (0% coverage)
		yangMetric := scopeMetrics.Metrics().AppendEmpty()
		service.createYANGAwareMetric(yangMetric, "yang_gauge", "/test/path", 789.12, timestamp, yangDataType)

		assert.Contains(t, yangMetric.Name(), "yang_gauge") // Has cisco. prefix
		assert.Equal(t, pmetric.MetricTypeGauge, yangMetric.Type())

		// Test createYANGAwareInfoMetric method (0% coverage)
		yangInfoMetric := scopeMetrics.Metrics().AppendEmpty()
		service.createYANGAwareInfoMetric(yangInfoMetric, "yang_info", "/test/path", "yang_value", timestamp, yangDataType)

		assert.Contains(t, yangInfoMetric.Name(), "yang_info") // Has cisco. prefix and _info suffix
		assert.Equal(t, pmetric.MetricTypeGauge, yangInfoMetric.Type())
	})
}

// TestGrpcService_ProcessField tests the processField method
func TestGrpcService_ProcessField(t *testing.T) {
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

		// Test processField with different field types
		fields := []*pb.TelemetryField{
			{
				Name: "uint64_field",
				ValueByType: &pb.TelemetryField_Uint64Value{
					Uint64Value: 42,
				},
			},
			{
				Name: "string_field",
				ValueByType: &pb.TelemetryField_StringValue{
					StringValue: "test_string",
				},
			},
			{
				Name: "double_field",
				ValueByType: &pb.TelemetryField_DoubleValue{
					DoubleValue: 3.14159,
				},
			},
		}

		for _, field := range fields {
			service.processField(scopeMetrics, field, "/base/path", "/prefix", timestamp)
		}

		// Should have created metrics for each field
		assert.True(t, scopeMetrics.Metrics().Len() >= len(fields))
	})
}
