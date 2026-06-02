# =============================================================================
# Stage 1: сборка бинарника
# =============================================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /cr-assistant ./cmd/review

# =============================================================================
# Stage 2: alpine вместо scratch — нужен /bin/sh для GitLab CI script
#
# scratch-образ не работает с GitLab docker executor: runner не может
# выполнить секцию script (нет shell), переменные окружения не пробрасываются.
# Alpine добавляет ~8 МБ но решает проблему полностью.
# =============================================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

# Копируем только бинарь — всё остальное уже есть в alpine
COPY --from=builder /cr-assistant /cr-assistant

ENTRYPOINT ["/cr-assistant"]