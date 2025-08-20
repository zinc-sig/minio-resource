# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/

# Build the binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o check ./cmd/check
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o in ./cmd/in
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o out ./cmd/out

# Test stage (optional)
FROM builder AS test
COPY scripts/ ./scripts/
# Run tests if they exist
RUN if [ -f scripts/test.sh ]; then sh scripts/test.sh; fi

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates bash

# Copy the binaries from builder
COPY --from=builder /build/check /opt/resource/check
COPY --from=builder /build/in /opt/resource/in
COPY --from=builder /build/out /opt/resource/out

# Make binaries executable
RUN chmod +x /opt/resource/check /opt/resource/in /opt/resource/out

WORKDIR /opt/resource
