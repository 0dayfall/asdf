# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o webfinger-server ./cmd/webfinger-server

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates curl
WORKDIR /app
COPY --from=builder /app/webfinger-server .
COPY --from=builder /app/web ./web
COPY --from=builder /app/data ./data
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/internal/migrations ./internal/migrations
EXPOSE 8080 9090
CMD ["/app/webfinger-server"]
