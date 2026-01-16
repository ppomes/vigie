# Multi-stage build for Go API service

# Stage 1: Builder
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files first
COPY go.mod ./
# Copy go.sum if it exists (optional)
COPY go.sum* ./

# Copy source code (needed for go mod tidy to detect all imports)
COPY . .

# Download dependencies and ensure go.sum is properly populated
RUN go mod download && go mod tidy

# Build the application
# CGO_ENABLED=0 for static binary, GOOS=linux for Linux target
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api/main.go

# Stage 2: Runtime
FROM alpine:latest

# Install ca-certificates for HTTPS requests to email providers
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/api .

# Copy migrations for database setup
COPY --from=builder /app/migrations ./migrations

# Copy .env.example as reference (actual .env should be mounted or use env vars)
COPY --from=builder /app/.env.example .

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/status || exit 1

# Run the application
CMD ["./api"]
