// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package ciscotelemetryreceiver provides an OpenTelemetry Collector receiver
// for Cisco IOS XE telemetry data via gRPC dial-out with kvGPB encoding.
// It includes an RFC 6020/7950 compliant YANG parser for dynamic schema discovery.
package ciscotelemetryreceiver // import "github.com/jcohoe/otel-grpc-cisco-receiver/receiver/ciscotelemetryreceiver"
