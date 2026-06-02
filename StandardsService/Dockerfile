# =============================================================================
# Stage 1: сборка бинарника
# =============================================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Кешируем зависимости отдельным слоем
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники и собираем статический бинарник
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /standards-service ./cmd/server

# =============================================================================
# Stage 2: минимальный runtime-образ
# =============================================================================
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /standards-service /standards-service
# Миграции должны быть доступны в runtime (путь "file://migrations")
COPY --from=builder /build/migrations /migrations

EXPOSE 9090

ENTRYPOINT ["/standards-service"]