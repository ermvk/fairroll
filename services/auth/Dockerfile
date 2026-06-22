# Dockerfile for auth service
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY pkg ./pkg
COPY services/auth ./services/auth

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /build/auth ./services/auth/cmd/main.go


FROM alpine:3.20

WORKDIR /app

COPY --from=builder /build/auth /app/

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

EXPOSE 8080

ENTRYPOINT ["/app/auth"]