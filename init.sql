CREATE TABLE IF NOT EXISTS payments (
    correlation_id UUID PRIMARY KEY,
    amount NUMERIC(10, 2) NOT NULL,
    processor CHAR(1) NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_processor ON payments (processor);
CREATE INDEX IF NOT EXISTS idx_requested_at ON payments (requested_at);
CREATE INDEX IF NOT EXISTS idx_processor_requested_at ON payments (processor, requested_at);
