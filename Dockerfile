# Stage 1: The Builder Stage
# Use a full Go image for compiling
FROM golang:1.25.4-alpine AS builder

WORKDIR /app

# Copy go mod and go sum files
COPY go.mod go.sum ./
# Download dependencies (if any)
RUN go mod download

# Copy the server code and static assets
COPY server.go .
COPY static static

# Compile the Go server. CGO_ENABLED=0 creates a static binary for a smaller, safer image.
RUN CGO_ENABLED=0 go build -o /server server.go

# Stage 2: The Final Stage (Small Alpine image)
FROM alpine:latest
# Install CA certificates for trust
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /app

# Copy the compiled server executable from the builder stage
COPY --from=builder /server /server
# Copy the static assets
COPY static static

# Cloud Run environment variable. Must be non-privileged (e.g., 8080).
ENV PORT=8080

# Command to run the executable
CMD ["/server"]