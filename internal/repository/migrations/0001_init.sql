CREATE TABLE IF NOT EXISTS notifications (
    id         TEXT PRIMARY KEY,
    channel    TEXT NOT NULL,
    recipient  TEXT NOT NULL,
    message    TEXT NOT NULL,
    status     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications (status);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications (created_at);
