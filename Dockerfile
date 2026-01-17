# Multi-architecture build stage
# For standard build, use: docker build -t k13s .
# For pre-built binary, use: docker build -f Dockerfile.prebuilt -t k13s .

FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o k13s ./cmd/kube-ai-dashboard-cli/main.go

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN adduser -D -g '' k13s

# Copy binary from builder
COPY --from=builder /app/k13s /usr/local/bin/k13s

# Create config and kubeconfig directories
RUN mkdir -p /home/k13s/.config/k13s /home/k13s/.kube && \
    chown -R k13s:k13s /home/k13s

# Switch to non-root user
USER k13s
WORKDIR /home/k13s

# Environment variables for configuration
# K13S_AUTH_MODE: local (username/password) or token (k8s token)
# K13S_USERNAME/K13S_PASSWORD: credentials for local auth mode
# KUBECONFIG: path to kubeconfig file (mount as volume)
ENV K13S_PORT=8080 \
    K13S_AUTH_MODE=local \
    K13S_USERNAME=admin \
    K13S_PASSWORD=admin

# Expose web UI port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -sf http://localhost:${K13S_PORT}/api/health || exit 1

# Default command: run in web mode
# Override with: docker run k13s ./k13s (for TUI mode with -it)
ENTRYPOINT ["/usr/local/bin/k13s"]
CMD ["-web", "-port", "8080"]
