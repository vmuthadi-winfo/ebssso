FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ebssso .

# Final stage
FROM alpine:latest

# Install runtime dependencies for Oracle client
RUN apk --no-cache add ca-certificates libaio libnsl libc6-compat

# Create app directory
WORKDIR /root/

# Copy binary and templates from builder
COPY --from=builder /app/ebssso .
COPY --from=builder /app/templates ./templates

# Expose port
EXPOSE 8080

# Run the application
CMD ["./ebssso"]
