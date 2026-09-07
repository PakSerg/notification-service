# NotiHub

Небольшой сервис отправки уведомлений: HTTP API, хранилище на PostgreSQL, подключаемые каналы доставки (email / webhook / push).

## Доставка

POST /notifications сразу возвращает уведомление со статусом pending, а сама отправка через нужный канал идёт в фоне. Если попытка неудачна, сервис повторяет её с экспоненциальной задержкой (jitter, чтобы не устраивать retry storm), пока не исчерпает лимит попыток. После каждой финальной попытки статус обновляется на sent или failed; актуальное состояние, число попыток (attempts) и текст последней ошибки (last_error) можно посмотреть через GET /notifications/{id}.

Политика повторов настраивается через окружение:

- NOTIHUB_RETRY_MAX_ATTEMPTS — общее число попыток доставки, включая первую, по умолчанию 3
- NOTIHUB_RETRY_BASE_DELAY — задержка перед второй попыткой, удваивается после каждого следующего провала, по умолчанию 500ms
- NOTIHUB_RETRY_MAX_DELAY — потолок задержки между попытками, по умолчанию 10s

## Запуск локально

Нужен PostgreSQL. Быстрее всего поднять только базу через compose и запустить сервис на хосте:

```bash
docker compose up -d db
NOTIHUB_DATABASE_URL="postgres://notihub:notihub@localhost:5432/notihub?sslmode=disable" go run ./cmd/notihub
```

Сервис поднимется на :8080.

## Конфигурация

Все настройки читаются из окружения, смотри internal/config:

- NOTIHUB_HTTP_ADDR — адрес HTTP-сервера, по умолчанию :8080
- NOTIHUB_DATABASE_URL — DSN подключения к PostgreSQL, по умолчанию postgres://notihub:notihub@localhost:5432/notihub?sslmode=disable
- NOTIHUB_SHUTDOWN_TIMEOUT — сколько ждать завершения активных запросов при остановке, по умолчанию 5s

## Хранилище

Данные лежат в PostgreSQL. Драйвер — jackc/pgx (через database/sql совместимость pgx/v5/stdlib), чистый Go без CGO, так что бинарник по-прежнему собирается статически.

Схема лежит в internal/repository/migrations и вшита в бинарник через embed. Непримененные миграции накатываются при старте и фиксируются в schema_migrations.

Интеграционные тесты репозитория (internal/repository/postgres_test.go) требуют реальной базы и включаются переменной NOTIHUB_TEST_DATABASE_URL; без неё они пропускаются.

## Деплой

Образ собирается многостадийно: сборка на golang:alpine, рантайм — alpine с непривилегированным пользователем. compose.yaml поднимает сервис вместе с PostgreSQL (данные — в отдельном именованном томе).

```bash
docker compose up --build -d
```

Или напрямую (тогда PostgreSQL нужно поднять и указать отдельно через NOTIHUB_DATABASE_URL):

```bash
docker build --build-arg VERSION=$(git describe --tags --always) -t notihub:local .
```

Полезные команды собраны в Makefile: make build, make test, make test-integration (поднимает PostgreSQL в docker compose и гоняет тесты репозитория), make lint, make docker-up, make docker-down.

Контейнер отвечает на SIGTERM graceful-shutdown'ом, healthcheck ходит в /health.

## CI

.github/workflows/ci.yml на каждый push и pull request поднимает сервис PostgreSQL, проверяет формат, прогоняет go vet, тесты с race-детектором (включая интеграционные тесты репозитория) и собирает Docker-образ.
