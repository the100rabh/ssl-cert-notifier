# Stage 1: Build the Go application
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first to cache dependencies
COPY go.mod ./
COPY go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the application
# CGO_ENABLED=0 is important for static binaries
# -ldflags="-s -w" reduces the binary size
RUN CGO_ENABLED=0 go build -o /ssl-cert-notifier ./cmd/cert-notifier

# Stage 2: Create a minimal image to run the application
FROM alpine:latest

# Set the working directory
WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /ssl-cert-notifier .

# Copy the configuration file (it will be mounted as a volume in docker-compose, but good for local testing)
# We will primarily rely on docker-compose to mount the config, but this is a fallback.
COPY config.yaml .

# Expose any necessary ports (not directly used by this app, but good practice for services)
# EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["/root/ssl-cert-notifier"]

# Default command-line arguments (optional, can be overridden)
# CMD ["--config", "config.yaml"]
