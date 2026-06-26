# Dockerfile for transfer service
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY pkg ./pkg
COPY services/currency ./services/currency
COPY services/transfer ./services/transfer

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /build/transfer ./services/transfer/cmd/main.go


FROM alpine:3.20

WORKDIR /app

COPY --from=builder /build/transfer /app/

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

EXPOSE 8082

ENTRYPOINT ["/app/transfer"]
