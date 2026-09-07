package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/PakSerg/NotiHub/internal/notification"
)

// PostgresRepository persists notifications in a PostgreSQL database.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository opens a connection pool to dsn and applies pending
// migrations. The caller owns the repository and must Close it.
func NewPostgresRepository(ctx context.Context, dsn string) (*PostgresRepository, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

// Save stores the notification, overwriting an existing one with the same ID.
func (r *PostgresRepository) Save(ctx context.Context, n *notification.Notification) error {
	const query = `
		INSERT INTO notifications (id, channel, recipient, message, status, created_at, attempts, last_error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
		    channel    = excluded.channel,
		    recipient  = excluded.recipient,
		    message    = excluded.message,
		    status     = excluded.status,
		    created_at = excluded.created_at,
		    attempts   = excluded.attempts,
		    last_error = excluded.last_error`

	_, err := r.db.ExecContext(ctx, query,
		n.ID,
		string(n.Channel),
		n.Recipient,
		n.Message,
		string(n.Status),
		n.CreatedAt.UTC(),
		n.Attempts,
		n.LastError,
	)
	if err != nil {
		return fmt.Errorf("save notification %s: %w", n.ID, err)
	}

	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*notification.Notification, error) {
	const query = `
		SELECT id, channel, recipient, message, status, created_at, attempts, last_error
		FROM notifications
		WHERE id = $1`

	var (
		n       notification.Notification
		channel string
		status  string
	)

	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&n.ID, &channel, &n.Recipient, &n.Message, &status, &n.CreatedAt, &n.Attempts, &n.LastError)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("get notification %s: %w", id, err)
	}

	n.Channel = notification.Channel(channel)
	n.Status = notification.Status(status)
	n.CreatedAt = n.CreatedAt.UTC()

	return &n, nil
}

func (r *PostgresRepository) UpdateDeliveryResult(ctx context.Context, id string, status notification.Status, attempts int, lastErr string) error {
	const query = `UPDATE notifications SET status = $1, attempts = $2, last_error = $3 WHERE id = $4`

	res, err := r.db.ExecContext(ctx, query, string(status), attempts, lastErr, id)
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
