# Subscription Service

REST-сервис для агрегации данных об онлайн-подписках пользователей.

## Стек технологий

- **Go** — основной язык
- **Gin** — HTTP-фреймворк
- **Uber Fx** — dependency injection
- **PostgreSQL 15** — база данных
- **pgx/v5** — драйвер PostgreSQL
- **golang-migrate** — миграции БД
- **swaggo** — Swagger-документация
- **slog** — структурированное логирование (стандартная библиотека)
- **Docker Compose** — контейнеризация

## Архитектура

Проект построен по принципам **Clean Architecture**:

```
.
├── cmd/app/               # Точка входа, сборка DI-контейнера
├── internal/
│   ├── domain/            # Сущности, DTO, интерфейсы (Repository, Service)
│   ├── repository/        # Реализация работы с PostgreSQL
│   │   └── postgres/
│   ├── service/           # Бизнес-логика
│   └── delivery/          # HTTP-хендлеры, middleware
│       └── http/v1/
├── migrations/            # SQL-миграции
├── docs/                  # Swagger (генерируется автоматически)
├── docker-compose.yml
├── Dockerfile
└── .env.example
```

## Быстрый старт

### 1. Клонировать репозиторий

```bash
git clone <url>
cd subscription_services
```

### 2. Настроить переменные окружения

```bash
cp .env.example .env
```

Отредактируй `.env` при необходимости:

```env
DB_USER=user_name
DB_PASSWORD=db_password
DB_NAME=db_name
DB_HOST=db_host
DB_PORT=db_port
```

### 3. Запустить через Docker Compose

```bash
docker compose up --build -d
```

Команда автоматически:
1. Поднимет контейнер с **PostgreSQL**
2. Накатит **миграции**
3. Соберет и запустит **Go-приложение**

Сервис будет доступен на `http://localhost:8080`.

### 4. Swagger UI

```
http://localhost:8080/swagger/index.html
```

### 5. Остановить сервис

```bash
# Остановить контейнеры (данные сохраняются)
docker compose down

# Остановить и удалить все данные (volume)
docker compose down -v
```

---

## API

### Формат дат

Все даты передаются в формате **`MM-YYYY`** (месяц-год).

Примеры: `07-2025`, `01-2024`, `12-2023`

### Эндпоинты

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/api/v1/subscriptions` | Создать подписку |
| `GET` | `/api/v1/subscriptions` | Список подписок (с пагинацией) |
| `GET` | `/api/v1/subscriptions/:id` | Получить подписку по ID |
| `PATCH` | `/api/v1/subscriptions/:id` | Обновить подписку |
| `DELETE` | `/api/v1/subscriptions/:id` | Удалить подписку |
| `GET` | `/api/v1/subscriptions/cost` | Подсчет стоимости за период |

---

### POST `/api/v1/subscriptions` — Создать подписку

**Тело запроса:**

```json
{
  "service_name": "Yandex Plus",
  "price": 400,
  "user_id": "60601fee-2bf1-4721-ae6f-7636e79a0cba",
  "start_date": "07-2025",
  "end_date": "12-2025"
}
```

> `end_date` — опциональное поле. Если не указано, подписка считается бессрочной.

**Ответ `201 Created`:**

```json
{
  "id": "a1b2c3d4-...",
  "service_name": "Yandex Plus",
  "price": 400,
  "user_id": "60601fee-2bf1-4721-ae6f-7636e79a0cba",
  "start_date": "07-2025",
  "end_date": "12-2025",
  "created_at": "2025-07-01T10:00:00Z",
  "updated_at": "2025-07-01T10:00:00Z"
}
```

---

### GET `/api/v1/subscriptions` — Список подписок

**Query-параметры:**

| Параметр | Тип | По умолчанию | Описание |
|---|---|---|---|
| `limit` | int | `10` | Количество записей (1–100) |
| `offset` | int | `0` | Смещение |

**Пример:** `GET /api/v1/subscriptions?limit=5&offset=0`

---

### GET `/api/v1/subscriptions/:id` — Получить подписку

**Пример:** `GET /api/v1/subscriptions/a1b2c3d4-...`

---

### PATCH `/api/v1/subscriptions/:id` — Обновить подписку

Все поля опциональны — обновляются только переданные.

```json
{
  "price": 599,
  "end_date": "06-2026"
}
```

Чтобы **очистить** дату окончания (сделать подписку бессрочной):

```json
{
  "end_date": ""
}
```

---

### DELETE `/api/v1/subscriptions/:id` — Удалить подписку

**Ответ `204 No Content`** — без тела.

---

### GET `/api/v1/subscriptions/cost` — Подсчет стоимости

Считает суммарную стоимость всех подписок пользователя за указанный период. Учитывает только те месяцы, в которые подписка была активна.

**Query-параметры:**

| Параметр | Тип | Обязательный | Описание |
|---|---|---|---|
| `user_id` | UUID | ✅ | ID пользователя |
| `start_period` | MM-YYYY | ✅ | Начало периода |
| `end_period` | MM-YYYY | ✅ | Конец периода |
| `service_name` | string | ❌ | Фильтр по названию сервиса |

**Пример:** `GET /api/v1/subscriptions/cost?user_id=60601fee-...&start_period=01-2025&end_period=12-2025`

**Ответ `200 OK`:**

```json
{
  "total_cost": 4800
}
```

> **Логика подсчета:** для каждой подписки вычисляется количество месяцев пересечения с запрошенным периодом, умноженное на месячную стоимость.

---

## Локальная разработка (без Docker)

```bash
# Установить зависимости
go mod download

# Обновить Swagger-документацию
~/go/bin/swag init -g cmd/app/main.go

# Запустить только БД
docker compose up db -d

# Запустить приложение
go run ./cmd/app/main.go
```

> При локальном запуске убедись, что в `.env` указан `DB_PORT=5433` (если БД запущена через Docker с маппингом порта `5433:5432`).
