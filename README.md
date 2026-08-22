# NotiHub

## Storage

Notifications are stored in a SQLite database file in the project root (`notihub.db`),
so the data survives restarts. The path can be changed with `NOTIHUB_DB_PATH`.

The driver is [modernc.org/sqlite](https://modernc.org/sqlite) — a pure Go
implementation, so no CGO and no C toolchain are required to build the project.

The schema lives in `internal/repository/migrations` and is embedded into the binary.
Pending migrations are applied automatically on start and tracked in `schema_migrations`.

## Run

```bash
go run ./cmd/notihub
```
