# Multi-stage build for smaller final image
FROM golang:1.24-alpine AS builder

# Install FFmpeg and other build dependencies
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

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cobblepod .

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

# Copy binary from builder stage
COPY --from=builder /app/cobblepod .

# Create data directory for temporary files and gcloud config directory
RUN mkdir -p /app/data && \
    mkdir -p /home/appuser/.config/gcloud && \
    chown -R appuser:appgroup /app && \
    chown -R appuser:appgroup /home/appuser && \
    chown -R appuser:appgroup /home/appuser/.config/gcloud

# Create volume mount point for gcloud config
VOLUME ["/home/appuser/.config/gcloud"]

# Switch to non-root user
USER appuser

# Expose port if needed (though this app doesn't seem to serve HTTP)
# EXPOSE 8080

# Need to get "/home/micah/.config/gcloud/application_default_credentials.json" into the container

CMD ["./cobblepod"]
