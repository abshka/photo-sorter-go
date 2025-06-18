# PhotoSorter Go Dockerfile
# Multi-stage build for smaller final image

# Build stage
FROM golang:1.21-alpine AS builder

# Install git for go modules
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o photo-sorter ./cmd/photo-sorter

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and ffmpeg for video processing
RUN apk --no-cache add ca-certificates ffmpeg

# Create non-root user
RUN addgroup -g 1001 -S photosorter && \
    adduser -u 1001 -S photosorter -G photosorter

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/photo-sorter .

# Copy example config
COPY --from=builder /app/config.example.yaml ./config.example.yaml

# Create directories for photos and logs
RUN mkdir -p /photos /logs && \
    chown -R photosorter:photosorter /app /photos /logs

# Switch to non-root user
USER photosorter

# Create volume mount points
VOLUME ["/photos", "/logs"]

# Set environment variables
ENV PHOTO_SORTER_LOGGING_FILE_PATH=/logs/photo-sorter.log
ENV PHOTO_SORTER_SOURCE_DIRECTORY=/photos

# Expose no ports (CLI application)

# Default command
ENTRYPOINT ["./photo-sorter"]
CMD ["--help"]