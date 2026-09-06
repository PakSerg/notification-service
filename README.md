# NotiHub

Небольшой сервис отправки уведомлений: HTTP API, хранилище на SQLite, подключаемые каналы доставки (email / webhook / push).

## Доставка

POST /notifications сразу возвращает уведомление со статусом pending, а сама отправка через нужный канал идёт в фоне. Если попытка неудачна, сервис повторяет её с экспоненциальной задержкой (jitter, чтобы не устраивать retry storm), пока не исчерпает лимит попыток. После каждой финальной попытки статус обновляется на sent или failed; актуальное состояние, число попыток (attempts) и текст последней ошибки (last_error) можно посмотреть через GET /notifications/{id}.

Политика повторов настраивается через окружение:

- NOTIHUB_RETRY_MAX_ATTEMPTS — общее число попыток доставки, включая первую, по умолчанию 3
- NOTIHUB_RETRY_BASE_DELAY — задержка перед второй попыткой, удваивается после каждого следующего провала, по умолчанию 500ms
- NOTIHUB_RETRY_MAX_DELAY — потолок задержки между попытками, по умолчанию 10s

## Запуск локально

```bash
go run ./cmd/notihub
```

Сервис поднимется на :8080 и создаст notihub.db в корне проекта.

## Конфигурация

Все настройки читаются из окружения, смотри internal/config:

- NOTIHUB_HTTP_ADDR — адрес HTTP-сервера, по умолчанию :8080
- NOTIHUB_DB_PATH — путь к файлу SQLite, по умолчанию notihub.db
- NOTIHUB_SHUTDOWN_TIMEOUT — сколько ждать завершения активных запросов при остановке, по умолчанию 5s

## Хранилище

Данные лежат в файле SQLite, поэтому переживают перезапуск. Драйвер — modernc.org/sqlite: чистый Go, без CGO, поэтому бинарник собирается статически и не тянет C-тулчейн.

Схема лежит в internal/repository/migrations и вшита в бинарник через embed. Непримененные миграции накатываются при старте и фиксируются в schema_migrations.

## Деплой

Образ собирается многостадийно: сборка на golang:alpine, рантайм — alpine с непривилегированным пользователем. База хранится в томе, смонтированном в /data.

```bash
docker compose up --build -d
```

Или напрямую:

```bash
docker build --build-arg VERSION=$(git describe --tags --always) -t notihub:local .
```

Полезные команды собраны в Makefile: make build, make test, make lint, make docker-up, make docker-down.

Контейнер отвечает на SIGTERM graceful-shutdown'ом, healthcheck ходит в /health.

## CI

.github/workflows/ci.yml на каждый push и pull request проверяет формат, прогоняет go vet, тесты с race-детектором и собирает Docker-образ.
