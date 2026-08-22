package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	_ "modernc.org/sqlite"

	"github.com/PakSerg/NotiHub/internal/notification"
)

// timeLayout is how timestamps are stored: SQLite has no native time type,
// so we keep them as UTC RFC3339 strings, which are also sortable as text.
const timeLayout = time.RFC3339Nano

// SQLiteRepository persists notifications in a SQLite database file.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository opens (or creates) the database file at path and applies
// pending migrations. The caller owns the repository and must Close it.
func NewSQLiteRepository(ctx context.Context, path string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite serializes writers, and WAL plus busy_timeout (see dsn) is enough
	// to let concurrent readers through without "database is locked" errors.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxIdleTime(time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &SQLiteRepository{db: db}, nil
}

// dsn builds a connection string with the pragmas we want on every connection.
func dsn(path string) string {
	params := url.Values{}
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_pragma", "foreign_keys(on)")
	params.Add("_pragma", "synchronous(NORMAL)")

	return "file:" + path + "?" + params.Encode()
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

// Save stores the notification, overwriting an existing one with the same ID.
func (r *SQLiteRepository) Save(ctx context.Context, n *notification.Notification) error {
	const query = `
		INSERT INTO notifications (id, channel, recipient, message, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
		    channel    = excluded.channel,
		    recipient  = excluded.recipient,
		    message    = excluded.message,
		    status     = excluded.status,
		    created_at = excluded.created_at`

	_, err := r.db.ExecContext(ctx, query,
		n.ID,
		string(n.Channel),
		n.Recipient,
		n.Message,
		string(n.Status),
		formatTime(n.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("save notification %s: %w", n.ID, err)
	}

	return nil
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (*notification.Notification, error) {
	const query = `
		SELECT id, channel, recipient, message, status, created_at
		FROM notifications
		WHERE id = ?`

	var (
		n         notification.Notification
		channel   string
		status    string
		createdAt string
	)

	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&n.ID, &channel, &n.Recipient, &n.Message, &status, &createdAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("get notification %s: %w", id, err)
	}

	n.Channel = notification.Channel(channel)
	n.Status = notification.Status(status)
	if n.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, fmt.Errorf("get notification %s: %w", id, err)
	}

	return &n, nil
}

func (r *SQLiteRepository) UpdateStatus(ctx context.Context, id string, status notification.Status) error {
	const query = `UPDATE notifications SET status = ? WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query, string(status), id)
	if err != nil {
		return fmt.Errorf("update status of notification %s: %w", id, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update status of notification %s: %w", id, err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t, nil
}
