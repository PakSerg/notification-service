package repository

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration is an immutable schema change identified by its file name.
// Files are applied in lexicographic order, so they are prefixed with a number.
type migration struct {
	name string
	sql  string
}

// migrate brings the database schema up to date. It is safe to call on every
// start: already applied migrations are recorded in schema_migrations and skipped.
func migrate(ctx context.Context, db *sql.DB) error {
	const createTable = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
		    name       TEXT PRIMARY KEY,
		    applied_at TIMESTAMPTZ NOT NULL
		)`

	if _, err := db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	for _, m := range migrations {
		applied, err := isApplied(ctx, db, m.name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := apply(ctx, db, m); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
	}

	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := fs.ReadFile(migrationsFS, "migrations/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		migrations = append(migrations, migration{name: e.Name(), sql: string(content)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].name < migrations[j].name })

	return migrations, nil
}

func isApplied(ctx context.Context, db *sql.DB, name string) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`

	var exists bool
	if err := db.QueryRowContext(ctx, query, name).Scan(&exists); err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return exists, nil
}

func apply(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range splitStatements(m.sql) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	const insert = `INSERT INTO schema_migrations (name, applied_at) VALUES ($1, $2)`
	if _, err := tx.ExecContext(ctx, insert, m.name, time.Now().UTC()); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// splitStatements breaks a migration file into individual SQL statements.
// Postgres' extended query protocol (used by pgx/database/sql) runs exactly
// one statement per Exec call, unlike SQLite which happily took a whole file
// at once - so a migration with more than one statement, like 0001_init.sql,
// must be sent one at a time. Migrations here are plain DDL with no
// semicolons inside string literals or function bodies, so a naive split on
// ";" is enough; it would not be for anything fancier (e.g. PL/pgSQL).
func splitStatements(sql string) []string {
	var stmts []string
	for _, s := range strings.Split(sql, ";") {
		if s = strings.TrimSpace(s); s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}
