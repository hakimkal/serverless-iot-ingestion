-- schema.sql (also in migrations/001_init.sql)
CREATE TABLE IF NOT EXISTS events (
                                      id         BIGSERIAL PRIMARY KEY,
                                      payload    JSONB NOT NULL,
                                      source     TEXT,
                                      received_at TIMESTAMPTZ DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS idx_events_received_at ON events(received_at);
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);


