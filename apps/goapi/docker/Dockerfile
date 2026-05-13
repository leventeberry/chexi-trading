# Build stage — pin minor.patch to match go.mod `go` directive; GOTOOLCHAIN can still fetch newer toolchain if needed.
FROM golang:1.25.10-alpine AS builder

# Satisfy go.mod `go` directive (e.g. 1.25.10) even when base image ships an older patch (needs network).
ENV GOTOOLCHAIN=auto

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod ./
COPY go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o goapi main.go

# Final runtime — pin Alpine minor (bump with rebuild + smoke test). See infra/docker/README.md.
FROM alpine:3.21

# Install ca-certificates for HTTPS requests; non-root runtime user
RUN apk --no-cache add ca-certificates \
    && addgroup -g 10001 app \
    && adduser -D -u 10001 -G app appuser

WORKDIR /app

# Copy the binary from builder
COPY --from=builder --chown=appuser:app /app/goapi .

# Copy swagger docs if they exist
COPY --from=builder --chown=appuser:app /app/api/openapi ./api/openapi

# SQL migrations (optional; used when USE_VERSIONED_MIGRATIONS=true)
COPY --from=builder --chown=appuser:app /app/migrations ./migrations

USER appuser

# Expose port
EXPOSE 8080

# Run the application
CMD ["./goapi"]
