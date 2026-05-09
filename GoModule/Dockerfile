# Stage 1: сборка бинарника
FROM golang:1.24.6-alpine AS builder

# Нужны git и ca-certificates для go mod download
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Копируем зависимости отдельным слоем — кешируется если go.mod не менялся
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники и собираем статический бинарник
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /cr-assistant ./cmd/review


# Stage 2: минимальный образ для запуска
FROM scratch

# ca-certificates нужны для HTTPS-запросов к GitLab и LLM API
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /cr-assistant /cr-assistant

# Все данные приходят через переменные окружения GitLab CI

ENTRYPOINT ["/cr-assistant"]