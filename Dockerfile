# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build both web and worker applications
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o web ./cmd/web
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker ./cmd/worker

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Create app user
RUN groupadd -r appgroup && useradd -r -g appgroup appuser

# Set working directory
WORKDIR /root/

# Copy binaries from builder
COPY --from=builder /app/web .
COPY --from=builder /app/worker .

# Change ownership
RUN chown -R appuser:appgroup /root/

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Set default metadata and content options as environment variables
ENV METADATA_TYPE=dynamodb
ENV METADATA_OPTIONS=""
ENV CONTENT_TYPE=s3
ENV CONTENT_OPTIONS=tritontube-video-content
ENV AWS_REGION=us-west-1

# Create data directory
USER root
RUN mkdir -p /data && chown appuser:appgroup /data
USER appuser

# Run the application (default to web server)
CMD ["sh", "-c", "./web -host 0.0.0.0 $METADATA_TYPE $METADATA_OPTIONS $CONTENT_TYPE $CONTENT_OPTIONS"]
