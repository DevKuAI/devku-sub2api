-- Small durable cursors bound maintenance work without introducing another task queue.
CREATE TABLE IF NOT EXISTS desktop_maintenance_progress (
 name TEXT PRIMARY KEY,
 next_date DATE,
 organization_id BIGINT NOT NULL DEFAULT 0,
 lease_token TEXT,
 lease_until TIMESTAMPTZ,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
