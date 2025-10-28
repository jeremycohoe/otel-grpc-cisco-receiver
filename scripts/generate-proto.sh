#!/bin/bash

# Generate protobuf Go files from proto definitions

set -e

# Install protobuf compiler and Go plugins if not already installed
if ! command -v protoc &> /dev/null; then
    echo "Installing protobuf compiler..."
    brew install protobuf
fi

if ! go list -m google.golang.org/protobuf &> /dev/null; then
    echo "Installing protobuf Go libraries..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Create output directory
mkdir -p proto/generated

echo "Generating Go files from proto definitions..."

# Generate from mdt_grpc_dialout.proto
protoc --go_out=proto/generated \
    --go_opt=paths=source_relative \
    --go-grpc_out=proto/generated \
    --go-grpc_opt=paths=source_relative \
    proto/mdt_grpc_dialout.proto

# Generate from telemetry.proto
protoc --go_out=proto/generated \
    --go_opt=paths=source_relative \
    proto/telemetry.proto

echo "Proto generation complete!"
echo "Generated files:"
find proto/generated -name "*.go" -type f