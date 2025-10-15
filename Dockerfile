# Multi-stage build for Go application
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mail-stress-test ./cmd/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/mail-stress-test .

# Copy config directory
COPY --from=builder /app/config ./config

# Create reports directory
RUN mkdir -p ./reports

# Expose port if needed (though this is a CLI tool, not a server)
# EXPOSE 8080

# Default command
CMD ["./mail-stress-test"]