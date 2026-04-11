package ciscotelemetryreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYANGDataTypes(t *testing.T) {
	parser := NewYANGParser()
	parser.LoadBuiltinModules()

	t.Run("TestDataTypeRetrieval", func(t *testing.T) {
		// Test interface statistics data types
		testCases := []struct {
			encodingPath string
			fieldName    string
			expectedType string
			expectedUnit string
			description  string
		}{
			{
				encodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
				fieldName:    "in-octets",
				expectedType: "uint64",
				expectedUnit: "bytes",
				description:  "Should identify in-octets as uint64 bytes counter",
			},
			{
				encodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
				fieldName:    "rx-pps",
				expectedType: "uint32",
				expectedUnit: "packets-per-second",
				description:  "Should identify rx-pps as uint32 rate",
			},
			{
				encodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
				fieldName:    "name",
				expectedType: "string",
				expectedUnit: "",
				description:  "Should identify name as string",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				dataType := parser.GetDataTypeForEncodingPath(tc.encodingPath, tc.fieldName)
				require.NotNil(t, dataType, "Data type should not be nil for %s", tc.fieldName)

				assert.Equal(t, tc.expectedType, dataType.Type, "Expected type %s for %s", tc.expectedType, tc.fieldName)
				assert.Equal(t, tc.expectedUnit, dataType.Units, "Expected unit %s for %s", tc.expectedUnit, tc.fieldName)

				t.Logf("Field %s: Type=%s, Units=%s, Desc=%s", tc.fieldName, dataType.Type, dataType.Units, dataType.Description)
			})
		}
	})

	t.Run("TestCPUProcessDataTypes", func(t *testing.T) {
		// Test CPU process data types
		encodingPath := "Cisco-IOS-XE-process-cpu-oper:cpu-usage/cpu-utilization"

		dataType := parser.GetDataTypeForEncodingPath(encodingPath, "five-seconds")
		require.NotNil(t, dataType, "Data type should not be nil for five-seconds")

		assert.Equal(t, "uint8", dataType.Type)
		assert.Equal(t, "percent", dataType.Units)
		assert.NotNil(t, dataType.Range)
		assert.Equal(t, int64(0), *dataType.Range.Min)
		assert.Equal(t, int64(100), *dataType.Range.Max)

		t.Logf("CPU five-seconds: Type=%s, Units=%s, Range=%d-%d",
			dataType.Type, dataType.Units, *dataType.Range.Min, *dataType.Range.Max)
	})

	t.Run("TestSemanticTypeClassification", func(t *testing.T) {
		// Test counter type identification
		counterField := parser.GetDataTypeForEncodingPath(
			"Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
			"in-octets",
		)
		require.NotNil(t, counterField)
		assert.True(t, counterField.IsCounterType(), "in-octets should be identified as counter type")
		assert.False(t, counterField.IsGaugeType(), "in-octets should not be identified as gauge type")

		// Test gauge type identification
		gaugeField := parser.GetDataTypeForEncodingPath(
			"Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
			"rx-pps",
		)
		require.NotNil(t, gaugeField)
		assert.True(t, gaugeField.IsGaugeType(), "rx-pps should be identified as gauge type")
		assert.False(t, gaugeField.IsCounterType(), "rx-pps should not be identified as counter type")

		// Test percentage gauge type
		percentField := parser.GetDataTypeForEncodingPath(
			"Cisco-IOS-XE-process-cpu-oper:cpu-usage/cpu-utilization",
			"five-seconds",
		)
		require.NotNil(t, percentField)
		assert.True(t, percentField.IsGaugeType(), "five-seconds should be identified as gauge type")
		assert.False(t, percentField.IsCounterType(), "five-seconds should not be identified as counter type")

		t.Logf("Counter identification: in-octets=%t, rx-pps=%t, five-seconds=%t",
			counterField.IsCounterType(), gaugeField.IsCounterType(), percentField.IsCounterType())
		t.Logf("Gauge identification: in-octets=%t, rx-pps=%t, five-seconds=%t",
			counterField.IsGaugeType(), gaugeField.IsGaugeType(), percentField.IsGaugeType())
	})

	t.Run("TestNumericTypeIdentification", func(t *testing.T) {
		// Test numeric type identification
		uint64Field := parser.GetDataTypeForEncodingPath(
			"Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
			"in-octets",
		)
		require.NotNil(t, uint64Field)
		assert.True(t, uint64Field.IsNumericType(), "uint64 should be identified as numeric")

		uint8Field := parser.GetDataTypeForEncodingPath(
			"Cisco-IOS-XE-process-cpu-oper:cpu-usage/cpu-utilization",
			"five-seconds",
		)
		require.NotNil(t, uint8Field)
		assert.True(t, uint8Field.IsNumericType(), "uint8 should be identified as numeric")

		stringField := parser.GetDataTypeForEncodingPath(
			"Cisco-IOS-XE-interfaces-oper:interfaces/interface",
			"name",
		)
		require.NotNil(t, stringField)
		assert.False(t, stringField.IsNumericType(), "string should not be identified as numeric")
	})

	t.Run("TestModuleCount", func(t *testing.T) {
		modules := parser.GetAvailableModules()
		// Should now include the process-cpu-oper module
		assert.Contains(t, modules, "Cisco-IOS-XE-process-cpu-oper")
		assert.GreaterOrEqual(t, len(modules), 4, "Should have at least 4 modules loaded")

		t.Logf("Available modules: %v", modules)
	})
}

func TestYANGDataTypeIntegration(t *testing.T) {
	// Test integration with the actual telemetry processing
	parser := NewYANGParser()
	parser.LoadBuiltinModules()

	t.Run("TestRealTelemetryPaths", func(t *testing.T) {
		// Test with the actual encoding paths we see in live telemetry
		realPaths := []struct {
			encodingPath   string
			expectedFields map[string]string // field -> expected type
		}{
			{
				encodingPath: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics",
				expectedFields: map[string]string{
					"name":             "string",
					"in-octets":        "uint64",
					"out-octets":       "uint64",
					"in-unicast-pkts":  "uint64",
					"out-unicast-pkts": "uint64",
					"rx-pps":           "uint32",
					"tx-pps":           "uint32",
					"rx-kbps":          "uint32",
					"tx-kbps":          "uint32",
					"in-discards":      "uint32",
					"out-discards":     "uint32",
					"in-errors":        "uint32",
					"out-errors":       "uint32",
				},
			},
		}

		for _, testPath := range realPaths {
			for fieldName, expectedType := range testPath.expectedFields {
				dataType := parser.GetDataTypeForEncodingPath(testPath.encodingPath, fieldName)
				if assert.NotNil(t, dataType, "Should find data type for %s", fieldName) {
					assert.Equal(t, expectedType, dataType.Type, "Expected type %s for field %s", expectedType, fieldName)
					if dataType.Type == "uint64" || dataType.Type == "int64" || dataType.Type == "float64" {
						t.Logf("* %s: %s (%s) - %s", fieldName, dataType.Type, dataType.Units, dataType.Description)
					}
				}
			}
		}
	})

	t.Run("TestUnitCorrectness", func(t *testing.T) {
		// Test that units are correctly assigned
		testCases := []struct {
			fieldName    string
			expectedUnit string
		}{
			{"in-octets", "bytes"},
			{"out-octets", "bytes"},
			{"in-unicast-pkts", "packets"},
			{"rx-pps", "packets-per-second"},
			{"tx-kbps", "kilobits-per-second"},
			{"in-discards", "packets"},
		}

		encodingPath := "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics"
		for _, tc := range testCases {
			dataType := parser.GetDataTypeForEncodingPath(encodingPath, tc.fieldName)
			if assert.NotNil(t, dataType, "Should find data type for %s", tc.fieldName) {
				assert.Equal(t, tc.expectedUnit, dataType.Units, "Expected unit %s for field %s", tc.expectedUnit, tc.fieldName)
			}
		}
	})
}
