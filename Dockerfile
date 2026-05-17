# СТАДИЯ 1: Сборка
FROM dockerhub.timeweb.cloud/library/golang:1.24-bullseye AS builder

RUN apt-get update && apt-get install -y libpcap-dev gcc

WORKDIR /app

# Настройка прокси
ENV GOPROXY=https://proxy.golang.org,https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Собираем бинарник с именем 'gateway_app' в корень /app
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/gateway_app ./gateway/main.go

# СТАДИЯ 2: Запуск
FROM dockerhub.timeweb.cloud/library/debian:bullseye-slim

RUN apt-get update && apt-get install -y libpcap0.8 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем бинарник из стадии builder
COPY --from=builder /app/gateway_app /app/gateway_app

# Даем права на запуск
RUN chmod +x /app/gateway_app

# Запускаем через абсолютный путь
ENTRYPOINT ["/app/gateway_app"]