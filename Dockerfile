# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o /app/bin/pipeliner ./cmd/pipeliner

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    bash

# Create app user
RUN addgroup -g 1000 pipeliner && \
    adduser -D -u 1000 -G pipeliner pipeliner

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/pipeliner /app/pipeliner

# Copy static files and templates
COPY --from=builder /app/static /app/static
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/config /app/config

# Create scans directory
RUN mkdir -p /app/scans && chown -R pipeliner:pipeliner /app

# Switch to app user
USER pipeliner

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3000/ || exit 1

# Run the application
CMD ["/app/pipeliner", "serve"]
