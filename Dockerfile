# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o k13s ./cmd/kube-ai-dashboard-cli/main.go

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' k13s

# Copy binary from builder
COPY --from=builder /app/k13s /usr/local/bin/k13s

# Create config directory
RUN mkdir -p /home/k13s/.config/k13s && \
    chown -R k13s:k13s /home/k13s

# Switch to non-root user
USER k13s
WORKDIR /home/k13s

# Expose web UI port
EXPOSE 8080

# Default command: run in web mode
# Override with: docker run k13s ./k13s (for TUI mode with -it)
ENTRYPOINT ["/usr/local/bin/k13s"]
CMD ["-web", "-port", "8080"]
