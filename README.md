# NotiHub

Небольшой сервис отправки уведомлений: HTTP API, хранилище на SQLite,
подключаемые каналы доставки (email / webhook / push).

## Запуск локально

```bash
go run ./cmd/notihub
```

Сервис поднимется на `:8080` и создаст `notihub.db` в корне проекта.

## Конфигурация

Все настройки читаются из окружения (см. `internal/config`):

| Переменная                 | По умолчанию | Описание                                  |
|----------------------------|--------------|-------------------------------------------|
| `NOTIHUB_HTTP_ADDR`        | `:8080`      | адрес HTTP-сервера                        |
| `NOTIHUB_DB_PATH`          | `notihub.db` | путь к файлу SQLite                       |
| `NOTIHUB_SHUTDOWN_TIMEOUT` | `5s`         | сколько ждать завершения активных запросов |

## Хранилище

Данные лежат в файле SQLite, поэтому переживают перезапуск. Драйвер —
[modernc.org/sqlite](https://modernc.org/sqlite): чистый Go, без CGO,
поэтому бинарник собирается статически и не тянет C-тулчейн.

Схема лежит в `internal/repository/migrations` и вшита в бинарник через `embed`.
Непримененные миграции накатываются при старте и фиксируются в `schema_migrations`.

## Деплой

Образ собирается многостадийно: сборка на `golang:alpine`, рантайм — `alpine`
с непривилегированным пользователем. База хранится в томе, смонтированном в `/data`.

```bash
docker compose up --build -d
```

Или напрямую:

```bash
docker build --build-arg VERSION=$(git describe --tags --always) -t notihub:local .
```

Полезные команды собраны в `Makefile`: `make build`, `make test`, `make lint`,
`make docker-up`, `make docker-down`.

Контейнер отвечает на `SIGTERM` graceful-shutdown'ом, healthcheck ходит в `/health`.

## CI

`.github/workflows/ci.yml` на каждый push и pull request проверяет формат,
прогоняет `go vet`, тесты с race-детектором и собирает Docker-образ.
