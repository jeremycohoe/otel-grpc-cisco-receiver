# Build stage: compile the custom OTel Collector using ocb (builder)
FROM golang:1.24-bookworm AS builder

WORKDIR /src

# Install OTel Collector Builder
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.138.0

# Copy module and builder config
COPY go.mod go.sum builder-config.yaml ./
COPY receiver/ receiver/
COPY proto/ proto/

# Run the builder to produce a single binary
RUN builder --config=builder-config.yaml

# Runtime stage: minimal image
FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=builder /src/build/cisco-otelcol /otelcol

EXPOSE 57500 8888 13133

ENTRYPOINT ["/otelcol"]
