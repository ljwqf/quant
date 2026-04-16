# Stage 1: Build
FROM golang:1.25-alpine AS builder

# GOPROXY for China
ENV GOPROXY=https://goproxy.cn,direct

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go.mod first to leverage layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o /app/okx-quant ./cmd/trader

# Stage 2: Runtime
FROM alpine:3.19

# Install runtime deps and create non-root user
RUN apk --no-cache add ca-certificates tzdata wget && \
    adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/okx-quant /app/

# Copy config templates and web assets
COPY --from=builder /app/configs /app/configs
COPY --from=builder /app/web /app/web

# Create runtime directories
RUN mkdir -p /app/logs /app/data/runtime && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Environment
ENV TZ=Asia/Shanghai \
    QUANT_ENV=simulation

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8765/health || exit 1

EXPOSE 8765

ENTRYPOINT ["/app/okx-quant"]
