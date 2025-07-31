CREATE TABLE IF NOT EXISTS payments (
    correlation_id UUID PRIMARY KEY,
    amount INTEGER NOT NULL,
    processor CHAR(1) NOT NULL CHECK (processor IN ('d', 'f')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_processor ON payments (processor);
CREATE INDEX IF NOT EXISTS idx_created_at ON payments (created_at);
CREATE INDEX IF NOT EXISTS idx_processor_created_at ON payments (processor, created_at);
