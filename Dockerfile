# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for dependencies if needed (though go mod download should be enough usually)
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the binary
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o walship ./cmd/walship

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/walship /usr/local/bin/walship

# Create a non-root user
RUN addgroup -S walship && adduser -S walship -G walship
USER walship

# Default entrypoint
ENTRYPOINT ["walship"]
