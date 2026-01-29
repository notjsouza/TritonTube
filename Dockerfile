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

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/web

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates ffmpeg

# Create app user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set working directory
WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

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

# Run the application
CMD ["sh", "-c", "./main -host 0.0.0.0 $METADATA_TYPE $METADATA_OPTIONS $CONTENT_TYPE $CONTENT_OPTIONS"]
