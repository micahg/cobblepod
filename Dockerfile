# Multi-stage build for smaller final image
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build both binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cobblepod-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cobblepod-worker ./cmd/worker

# Final stage - minimal runtime image
FROM alpine:latest

# Install FFmpeg and certificates
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata

# Create app user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Create app directory
WORKDIR /app

# Copy both binaries from builder stage
COPY --from=builder /app/cobblepod-server .
COPY --from=builder /app/cobblepod-worker .

# Create data directory for temporary files and gcloud config directory
RUN mkdir -p /app/data && \
    mkdir -p /home/appuser/.config/gcloud && \
    chown -R appuser:appgroup /app && \
    chown -R appuser:appgroup /home/appuser && \
    chown -R appuser:appgroup /home/appuser/.config/gcloud

# Switch to non-root user
USER appuser

# Expose server port
EXPOSE 8080

# Default to running the server (can be overridden in docker-compose)
CMD ["./cobblepod-server"]
