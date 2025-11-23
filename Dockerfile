FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o walship ./cmd/walship

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata && \
    rm -rf /var/cache/apk/*

COPY --from=builder /app/walship /usr/local/bin/walship

RUN addgroup -S walship && adduser -S walship -G walship
USER walship

ENTRYPOINT ["walship"]