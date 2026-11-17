# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

# Final lightweight runtime image
FROM alpine:3.20

WORKDIR /app

RUN apk --no-cache add ca-certificates wget \
    && addgroup -S appgroup \
    && adduser -S appuser -G appgroup \
    && mkdir -p /app/data \
    && chown -R appuser:appgroup /app

COPY --from=builder --chown=appuser:appgroup /app/server /app/server

USER appuser

EXPOSE 8080

CMD ["/app/server"]

