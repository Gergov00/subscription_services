# Сборка
FROM golang:1.26.1-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем остальной код
COPY . .

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux go build -o /subscription_app ./cmd/app/main.go

# Финальный легковесный образ
FROM alpine:3.18

WORKDIR /app

# Копируем бинарник из стадии сборки
COPY --from=builder /subscription_app .

EXPOSE 8080

CMD ["./subscription_app"]
