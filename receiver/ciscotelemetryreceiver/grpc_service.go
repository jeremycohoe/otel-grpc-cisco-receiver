package ciscotelemetryreceiver

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/proto"
)

// grpcService implements the Cisco MDT gRPC service
type grpcService struct {
	pb.UnimplementedGRPCMdtDialoutServer
	receiver      *ciscoTelemetryReceiver
	yangParser    *YANGParser
	rfcYangParser *RFC6020Parser
}

// MdtDialout handles the bidirectional streaming gRPC call from Cisco devices
func (s *grpcService) MdtDialout(stream grpc.BidiStreamingServer[pb.MdtDialoutArgs, pb.MdtDialoutArgs]) error {
	ctx := stream.Context()
	clientAddr := "unknown"
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		clientAddr = p.Addr.String()
	}

	s.receiver.telemetryBuilder.RecordConnectionOpened(ctx, clientAddr)
	defer s.receiver.telemetryBuilder.RecordConnectionClosed(ctx, clientAddr)

	s.receiver.settings.Logger.Info("New MDT dialout connection established")

	for {
		// Receive telemetry data from Cisco device
		req, err := stream.Recv()
		if err == io.EOF {
			s.receiver.settings.Logger.Info("MDT dialout connection closed by client")
			return nil
		}
		if err != nil {
			s.receiver.telemetryBuilder.RecordGRPCError(ctx, "receive_error", "stream_recv")
			s.receiver.settings.Logger.Error("Error receiving MDT data", zap.Error(err))
			return err
		}

		// Process the received telemetry data
		err = s.processTelemetryData(req)
		if err != nil {
			s.receiver.telemetryBuilder.RecordGRPCError(ctx, "processing_error", "process_telemetry")
			s.receiver.settings.Logger.Error("Error processing telemetry data",
				zap.Error(err), zap.Int64("req_id", req.ReqId))

			// Send error response back to device
			resp := &pb.MdtDialoutArgs{
				ReqId:  req.ReqId,
				Errors: fmt.Sprintf("Processing error: %v", err),
			}
			if sendErr := stream.Send(resp); sendErr != nil {
				s.receiver.telemetryBuilder.RecordGRPCError(ctx, "send_error", "stream_send")
				s.receiver.settings.Logger.Error("Failed to send error response", zap.Error(sendErr))
			}
			continue
		}

		// Send acknowledgment back to device
		resp := &pb.MdtDialoutArgs{
			ReqId: req.ReqId,
		}
		if err := stream.Send(resp); err != nil {
			s.receiver.settings.Logger.Error("Failed to send acknowledgment", zap.Error(err))
			return err
		}
	}
}

// processTelemetryData parses the incoming telemetry data and converts to OTEL metrics
func (s *grpcService) processTelemetryData(req *pb.MdtDialoutArgs) error {
	ctx := context.Background()
	startTime := time.Now()

	// Record message received metrics
	nodeID := "unknown"
	subscriptionID := fmt.Sprintf("%d", req.ReqId)

	if len(req.Data) == 0 {
		s.receiver.telemetryBuilder.RecordMessageDropped(ctx, nodeID, subscriptionID, "empty_data")
		return fmt.Errorf("empty telemetry data")
	}

	s.receiver.telemetryBuilder.RecordMessageReceived(ctx, nodeID, subscriptionID, int64(len(req.Data)))

	// Parse the telemetry message from the data field
	telemetryMsg := &pb.Telemetry{}
	if err := proto.Unmarshal(req.Data, telemetryMsg); err != nil {
		s.receiver.telemetryBuilder.RecordMessageDropped(ctx, nodeID, subscriptionID, "unmarshal_error")
		return fmt.Errorf("failed to unmarshal telemetry data: %w", err)
	}

	// Update nodeID with actual value
	if telemetryMsg.GetNodeIdStr() != "" {
		nodeID = telemetryMsg.GetNodeIdStr()
	}

	// Convert to OTEL metrics
	metrics := s.convertToOTELMetrics(telemetryMsg)
	if metrics.MetricCount() == 0 {
		s.receiver.telemetryBuilder.RecordMessageDropped(ctx, nodeID, subscriptionID, "no_metrics_extracted")
		s.receiver.settings.Logger.Warn("No metrics extracted from telemetry data",
			zap.String("encoding_path", telemetryMsg.EncodingPath),
			zap.String("node_id", telemetryMsg.GetNodeIdStr()))
		return nil
	}

	// Send metrics to the consumer
	err := s.receiver.consumer.ConsumeMetrics(ctx, metrics)
	if err != nil {
		s.receiver.telemetryBuilder.RecordMessageDropped(ctx, nodeID, subscriptionID, "consumer_error")
		return fmt.Errorf("failed to consume metrics: %w", err)
	}

	// Record successful processing with timing
	processingDuration := time.Since(startTime)
	yangModule := s.extractYANGModule(telemetryMsg.EncodingPath)
	s.receiver.telemetryBuilder.RecordMessageProcessed(ctx, nodeID, subscriptionID, yangModule, processingDuration)

	s.receiver.settings.Logger.Debug("Successfully processed telemetry data",
		zap.Int64("req_id", req.ReqId),
		zap.String("encoding_path", telemetryMsg.EncodingPath),
		zap.Int("metric_count", metrics.MetricCount()),
		zap.Duration("processing_duration", processingDuration))

	return nil
}

// extractYANGModule extracts the YANG module name from encoding path
func (s *grpcService) extractYANGModule(encodingPath string) string {
	// Example: "Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics"
	if idx := strings.Index(encodingPath, ":"); idx != -1 {
		return encodingPath[:idx]
	}
	return "unknown"
}

// convertToOTELMetrics converts Cisco telemetry data to OpenTelemetry metrics format
func (s *grpcService) convertToOTELMetrics(telemetry *pb.Telemetry) pmetric.Metrics {
	// Use RFC YANG parser analysis for this encoding path to enrich
	// resource attributes with module and type information.
	if s.rfcYangParser != nil {
		rfcAnalysis := s.rfcYangParser.AnalyzeTelemetryPath(telemetry.EncodingPath)
		if rfcAnalysis != nil && rfcAnalysis.IsValid {
			s.receiver.settings.Logger.Debug("RFC YANG analysis",
				zap.String("encoding_path", telemetry.EncodingPath),
				zap.String("module", rfcAnalysis.ModuleName),
				zap.String("xpath", rfcAnalysis.XPath),
				zap.String("semantic_type", rfcAnalysis.SemanticType))
		}
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes
	resource := resourceMetrics.Resource()
	resourceAttrs := resource.Attributes()

	if nodeID := telemetry.GetNodeIdStr(); nodeID != "" {
		resourceAttrs.PutStr("cisco.node_id", nodeID)
	}
	if subscriptionID := telemetry.GetSubscriptionIdStr(); subscriptionID != "" {
		resourceAttrs.PutStr("cisco.subscription_id", subscriptionID)
	}
	resourceAttrs.PutStr("cisco.encoding_path", telemetry.EncodingPath)

	// Enrichment: add YANG module name and encoding type as resource attributes.
	yangModule := s.extractYANGModule(telemetry.EncodingPath)
	if yangModule != "unknown" {
		resourceAttrs.PutStr("cisco.yang_module", yangModule)
	}
	resourceAttrs.PutStr("cisco.encoding", "kvGPB")

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scope := scopeMetrics.Scope()
	scope.SetName("github.com/jcohoe/otel-grpc-cisco-receiver")
	scope.SetVersion("0.1.0")

	// Process kvGPB data if present
	if len(telemetry.DataGpbkv) > 0 {
		s.processKvGPBData(scopeMetrics, telemetry)
	}

	// Process GPB table data if present
	if telemetry.DataGpb != nil {
		s.processGPBTableData(scopeMetrics, telemetry)
	}

	return metrics
}

// processKvGPBData processes key-value GPB formatted telemetry data.
// It uses a two-pass approach: first extract YANG list keys from each top-level
// entry, then process all fields while propagating those keys as attributes on
// every sibling metric. This allows downstream systems (Splunk, Prometheus) to
// group/filter metrics by entity (interface name, process name, sensor name).
func (s *grpcService) processKvGPBData(scopeMetrics pmetric.ScopeMetrics, telemetry *pb.Telemetry) {
	timestamp := pcommon.Timestamp(telemetry.MsgTimestamp * 1000000) // Convert milliseconds to nanoseconds

	// Resolve which field names are keys for this encoding path.
	keyFieldNames := s.resolveKeyFields(telemetry.EncodingPath)

	for _, field := range telemetry.DataGpbkv {
		effectiveKeyFields := keyFieldNames

		// If no keys found at the encoding path level, try appending the
		// field name. This handles cases where the encoding path is a container
		// and each DataGpbkv entry is a list entry (e.g., encoding_path=
		// "environment-sensors", field.Name="environment-sensor").
		if len(effectiveKeyFields) == 0 && field.Name != "" && len(field.Fields) > 0 {
			childPath := telemetry.EncodingPath + "/" + field.Name
			effectiveKeyFields = s.resolveKeyFields(childPath)
		}

		// Pass 1 — collect key values from immediate children of this list entry.
		listKeys := s.extractListKeys(field, effectiveKeyFields)

		// Pass 2 — process fields, attaching keys to every data point.
		s.processField(scopeMetrics, field, telemetry.EncodingPath, "", timestamp, listKeys)
	}
}

// resolveKeyFields returns the set of key field names for an encoding path,
// consulting both the built-in YANG parser and the RFC parser.
func (s *grpcService) resolveKeyFields(encodingPath string) map[string]bool {
	keyNames := make(map[string]bool)

	// Try built-in YANG parser first
	analysis := s.yangParser.AnalyzeEncodingPath(encodingPath)
	if analysis != nil {
		for _, keyName := range analysis.Keys {
			keyNames[keyName] = true
		}
		// Also look up the ListKeys directly by module
		moduleName := analysis.ModuleName
		listPath := analysis.ListPath
		if keys := s.yangParser.GetKeysForList(moduleName, listPath); len(keys) > 0 {
			for _, k := range keys {
				keyNames[k] = true
			}
		}
	}

	// Supplement with RFC parser
	if s.rfcYangParser != nil {
		rfcAnalysis := s.rfcYangParser.AnalyzeTelemetryPath(encodingPath)
		if rfcAnalysis != nil && rfcAnalysis.IsValid {
			for _, k := range rfcAnalysis.ListKeys {
				keyNames[k] = true
			}
		}
	}

	return keyNames
}

// extractListKeys scans a kvGPB list entry for key values.
// In Cisco kvGPB, each list row has two top-level children: "keys" and "content".
// The actual key fields (e.g., "name") are nested inside the "keys" child.
// This function looks both at direct children AND inside a "keys" sub-field.
func (s *grpcService) extractListKeys(field *pb.TelemetryField, keyFieldNames map[string]bool) map[string]string {
	keys := make(map[string]string)

	for _, child := range field.Fields {
		// Case 1: The child IS a known key field (flat structure).
		if keyFieldNames[child.Name] {
			s.extractKeyValue(child, keys)
			continue
		}

		// Case 2: The child is the "keys" container — dig inside it.
		if child.Name == "keys" {
			for _, keyChild := range child.Fields {
				if keyFieldNames[keyChild.Name] {
					s.extractKeyValue(keyChild, keys)
				}
			}
		}
	}

	return keys
}

// extractKeyValue reads the typed value of a TelemetryField and stores it in
// the keys map under the field's name.
func (s *grpcService) extractKeyValue(field *pb.TelemetryField, keys map[string]string) {
	switch v := field.ValueByType.(type) {
	case *pb.TelemetryField_StringValue:
		keys[field.Name] = v.StringValue
	case *pb.TelemetryField_Uint32Value:
		keys[field.Name] = fmt.Sprintf("%d", v.Uint32Value)
	case *pb.TelemetryField_Uint64Value:
		keys[field.Name] = fmt.Sprintf("%d", v.Uint64Value)
	case *pb.TelemetryField_Sint32Value:
		keys[field.Name] = fmt.Sprintf("%d", v.Sint32Value)
	case *pb.TelemetryField_Sint64Value:
		keys[field.Name] = fmt.Sprintf("%d", v.Sint64Value)
	}
}

// processField recursively processes a telemetry field and creates metrics.
// listKeys carries the YANG list key values extracted from the parent entry so
// they can be attached as attributes on every sibling metric.
func (s *grpcService) processField(scopeMetrics pmetric.ScopeMetrics, field *pb.TelemetryField, basePath, pathPrefix string, timestamp pcommon.Timestamp, listKeys map[string]string) {
	currentPath := pathPrefix
	if currentPath != "" {
		currentPath += "."
	}
	currentPath += field.Name

	// YANG analysis is now done within the metric creation methods
	// If field has a value, create a metric
	if field.ValueByType != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()

		// Get YANG data type information for this field
		yangDataType := s.yangParser.GetDataTypeForEncodingPath(basePath, field.Name)

		switch value := field.ValueByType.(type) {
		case *pb.TelemetryField_Uint32Value:
			s.createYANGAwareMetric(metric, currentPath, basePath, float64(value.Uint32Value), timestamp, yangDataType, listKeys)
		case *pb.TelemetryField_Uint64Value:
			s.createYANGAwareMetric(metric, currentPath, basePath, float64(value.Uint64Value), timestamp, yangDataType, listKeys)
		case *pb.TelemetryField_Sint32Value:
			s.createYANGAwareMetric(metric, currentPath, basePath, float64(value.Sint32Value), timestamp, yangDataType, listKeys)
		case *pb.TelemetryField_Sint64Value:
			s.createYANGAwareMetric(metric, currentPath, basePath, float64(value.Sint64Value), timestamp, yangDataType, listKeys)
		case *pb.TelemetryField_DoubleValue:
			s.createYANGAwareMetric(metric, currentPath, basePath, value.DoubleValue, timestamp, yangDataType, listKeys)
		case *pb.TelemetryField_FloatValue:
			s.createYANGAwareMetric(metric, currentPath, basePath, float64(value.FloatValue), timestamp, yangDataType, listKeys)
		case *pb.TelemetryField_BoolValue:
			val := 0.0
			if value.BoolValue {
				val = 1.0
			}
			s.createYANGAwareMetric(metric, currentPath, basePath, val, timestamp, yangDataType, listKeys)
		case *pb.TelemetryField_StringValue:
			// For string values, create YANG-aware info metric
			s.createYANGAwareInfoMetric(metric, currentPath, basePath, value.StringValue, timestamp, yangDataType, listKeys)
		default:
			// Remove the metric we added if we can't handle the type
			scopeMetrics.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				return m.Name() == ""
			})
		}
	}

	// Process nested fields recursively, propagating list keys.
	// For each child, check if it's a nested list entry with its own keys.
	for _, nestedField := range field.Fields {
		nestedKeys := listKeys

		// If this child is a container (has children, no value), check whether
		// it's a YANG list entry with known keys. This handles deeply nested
		// lists like cpu-usage-processes/cpu-usage-process and arp-vrf/arp-oper.
		if len(nestedField.Fields) > 0 && nestedField.ValueByType == nil {
			candidatePath := s.buildChildEncodingPath(basePath, currentPath, nestedField.Name)
			childKeyNames := s.resolveKeyFields(candidatePath)
			if len(childKeyNames) > 0 {
				extracted := s.extractListKeys(nestedField, childKeyNames)
				if len(extracted) > 0 {
					// Merge parent keys with child keys
					merged := make(map[string]string, len(listKeys)+len(extracted))
					for k, v := range listKeys {
						merged[k] = v
					}
					for k, v := range extracted {
						merged[k] = v
					}
					nestedKeys = merged
				}
			}
		}

		s.processField(scopeMetrics, nestedField, basePath, currentPath, timestamp, nestedKeys)
	}
}

// processGPBTableData processes GPB table formatted telemetry data
func (s *grpcService) processGPBTableData(scopeMetrics pmetric.ScopeMetrics, telemetry *pb.Telemetry) {
	// For GPB table data, we would need specific protobuf definitions for each encoding path
	// This is a placeholder implementation
	s.receiver.settings.Logger.Debug("GPB table data processing not implemented",
		zap.String("encoding_path", telemetry.EncodingPath))
}

// buildChildEncodingPath constructs an encoding path for a nested field by
// combining the base encoding path with the dotted path prefix and child name.
// kvGPB uses "keys" and "content" structural wrappers that are not part of
// the YANG path, so those segments are stripped from the prefix.
func (s *grpcService) buildChildEncodingPath(basePath, dottedPrefix, childName string) string {
	parts := strings.SplitN(basePath, ":", 2)
	if len(parts) != 2 {
		return basePath + "/" + childName
	}
	module := parts[0]
	path := parts[1]

	if dottedPrefix != "" {
		// Strip kvGPB structural wrappers from the path
		segments := strings.Split(dottedPrefix, ".")
		filtered := make([]string, 0, len(segments))
		for _, seg := range segments {
			if seg != "keys" && seg != "content" {
				filtered = append(filtered, seg)
			}
		}
		if len(filtered) > 0 {
			path += "/" + strings.Join(filtered, "/")
		}
	}
	path += "/" + childName

	return module + ":" + path
}

// isKeyField checks if a field name is a key field based on YANG analysis
func (s *grpcService) isKeyField(fieldName string, analysis *PathAnalysis) bool {
	if analysis == nil {
		return false
	}

	// Check if this field matches any known key fields for the path
	for _, keyField := range analysis.Keys {
		if fieldName == keyField {
			return true
		}
	}

	// Additional heuristics for common key patterns
	commonKeys := []string{"name", "id", "index", "interface-name", "neighbor-id", "router-id"}
	for _, commonKey := range commonKeys {
		if fieldName == commonKey {
			return true
		}
	}

	return false
}

// extractFieldName extracts the field name from a metric name path
func (s *grpcService) extractFieldName(metricName string) string {
	// Remove common prefixes and get the last component
	parts := strings.Split(metricName, ".")
	if len(parts) == 0 {
		return metricName
	}

	// Get the last part, which is usually the field name
	fieldName := parts[len(parts)-1]

	// Remove common suffixes like "_info"
	if strings.HasSuffix(fieldName, "_info") {
		fieldName = strings.TrimSuffix(fieldName, "_info")
	}

	return fieldName
}

// createYANGAwareMetric creates a metric with YANG data type awareness.
// listKeys are the parent list's key-value pairs, added as attributes so
// downstream systems can group by entity (interface, process, sensor, etc.).
func (s *grpcService) createYANGAwareMetric(metric pmetric.Metric, name, encodingPath string, value float64, timestamp pcommon.Timestamp, yangDataType *YANGDataType, listKeys map[string]string) {
	// Determine metric name and type based on YANG information
	metricName := fmt.Sprintf("cisco.%s", name)
	metricDescription := fmt.Sprintf("Cisco telemetry metric from %s", encodingPath)
	metricUnit := "1" // Default unit

	if yangDataType != nil {
		// Use YANG-provided description and units
		if yangDataType.Description != "" {
			metricDescription = yangDataType.Description
		}
		if yangDataType.Units != "" {
			metricUnit = yangDataType.Units
		}

		// Add YANG type to metric name for clarity
		if yangDataType.Type != "" {
			metricName = fmt.Sprintf("cisco.%s", name)
		}
	}

	metric.SetName(metricName)
	metric.SetDescription(metricDescription)
	metric.SetUnit(metricUnit)

	// Determine if this should be a counter or gauge based on YANG data type
	if yangDataType != nil && yangDataType.IsCounterType() {
		// Create a sum metric for counters (monotonic increasing)
		sum := metric.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		dp := sum.DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
		dp.SetTimestamp(timestamp)

		// Add YANG list key attributes first, then YANG metadata.
		for k, v := range listKeys {
			dp.Attributes().PutStr(k, v)
		}
		s.addYANGAttributes(dp.Attributes(), encodingPath, yangDataType, name)

	} else {
		// Create a gauge metric for everything else
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
		dp.SetTimestamp(timestamp)

		// Add YANG list key attributes first, then YANG metadata.
		for k, v := range listKeys {
			dp.Attributes().PutStr(k, v)
		}
		s.addYANGAttributes(dp.Attributes(), encodingPath, yangDataType, name)
	}
}

// createYANGAwareInfoMetric creates an info metric with YANG data type awareness.
// listKeys are the parent list's key-value pairs, added as attributes.
func (s *grpcService) createYANGAwareInfoMetric(metric pmetric.Metric, name, encodingPath, value string, timestamp pcommon.Timestamp, yangDataType *YANGDataType, listKeys map[string]string) {
	metricName := fmt.Sprintf("cisco.%s_info", name)
	metricDescription := fmt.Sprintf("Cisco telemetry info metric from %s", encodingPath)

	if yangDataType != nil && yangDataType.Description != "" {
		metricDescription = yangDataType.Description
	}

	metric.SetName(metricName)
	metric.SetDescription(metricDescription)
	metric.SetUnit("1")

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.0) // Info metrics always have value 1
	dp.SetTimestamp(timestamp)

	// Add the string value as an attribute
	dp.Attributes().PutStr("value", value)

	// Add YANG list key attributes, then YANG metadata.
	for k, v := range listKeys {
		dp.Attributes().PutStr(k, v)
	}
	s.addYANGAttributes(dp.Attributes(), encodingPath, yangDataType, name)
}

// addYANGAttributes adds YANG-derived attributes to metric data points
func (s *grpcService) addYANGAttributes(attrs pcommon.Map, encodingPath string, yangDataType *YANGDataType, fieldName string) {
	// Always add encoding path
	attrs.PutStr("encoding_path", encodingPath)

	// Use RFC-compliant YANG parser for enhanced analysis
	var rfcAnalysis *RFC6020TelemetryAnalysis
	if s.rfcYangParser != nil {
		rfcAnalysis = s.rfcYangParser.AnalyzeTelemetryPath(encodingPath)
	}
	if rfcAnalysis != nil && rfcAnalysis.IsValid {
		if rfcAnalysis.ModuleName != "" {
			attrs.PutStr("yang.module", rfcAnalysis.ModuleName)
		}
		if rfcAnalysis.ListPath != "" {
			attrs.PutStr("yang.list_path", rfcAnalysis.ListPath)
		}
		if rfcAnalysis.SemanticType != "" {
			attrs.PutStr("yang.semantic_type", rfcAnalysis.SemanticType)
		}

		// Add list key information
		if len(rfcAnalysis.ListKeys) > 0 {
			// Check if current field is a key field
			for _, key := range rfcAnalysis.ListKeys {
				if strings.Contains(fieldName, key) {
					attrs.PutStr("yang.is_key", "true")
					attrs.PutStr("yang.key_type", key)
					break
				}
			}
		}

		// Add inferred data type from RFC parser
		if s.rfcYangParser != nil && len(rfcAnalysis.PathSegments) > 0 {
			leafName := rfcAnalysis.PathSegments[len(rfcAnalysis.PathSegments)-1]
			inferredType := s.rfcYangParser.InferDataTypeFromPath(leafName)
			if inferredType != nil && inferredType.Name != "" {
				attrs.PutStr("yang.data_type", inferredType.Name)
			}
		}
	}

	// Fallback to basic YANG parser analysis if RFC parser fails
	if rfcAnalysis == nil || !rfcAnalysis.IsValid {
		analysis := s.yangParser.AnalyzeEncodingPath(encodingPath)
		if analysis != nil {
			if analysis.ModuleName != "" {
				attrs.PutStr("yang.module", analysis.ModuleName)
			}
			if analysis.ListPath != "" {
				attrs.PutStr("yang.list_path", analysis.ListPath)
			}

			// Check if this is a key field
			if s.isKeyField(fieldName, analysis) {
				attrs.PutStr("yang.is_key", "true")
				attrs.PutStr("yang.key_type", fieldName)
			}
		}
	}

	// Add YANG data type information from basic parser
	if yangDataType != nil {
		if yangDataType.Type != "" {
			attrs.PutStr("yang.data_type", yangDataType.Type)
		}
		if yangDataType.Units != "" {
			attrs.PutStr("yang.units", yangDataType.Units)
		}
		if yangDataType.Description != "" {
			attrs.PutStr("yang.description", yangDataType.Description)
		}

		// Add semantic information
		if yangDataType.IsCounterType() {
			attrs.PutStr("yang.semantic_type", "counter")
		} else if yangDataType.IsGaugeType() {
			attrs.PutStr("yang.semantic_type", "gauge")
		}

		// Add range information if available
		if yangDataType.Range != nil {
			if yangDataType.Range.Min != nil {
				attrs.PutInt("yang.min_value", *yangDataType.Range.Min)
			}
			if yangDataType.Range.Max != nil {
				attrs.PutInt("yang.max_value", *yangDataType.Range.Max)
			}
		}
	}
}
