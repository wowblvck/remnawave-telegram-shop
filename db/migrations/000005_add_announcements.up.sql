-- Create announcement table
CREATE TABLE IF NOT EXISTS announcement (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by_admin_id BIGINT NOT NULL
);

-- Indexes for announcement
CREATE INDEX IF NOT EXISTS idx_announcement_expires_at ON announcement (expires_at);
CREATE INDEX IF NOT EXISTS idx_announcement_is_active ON announcement (is_active);
CREATE INDEX IF NOT EXISTS idx_announcement_created_at ON announcement (created_at);

-- Create announcement_delivery table
CREATE TABLE IF NOT EXISTS announcement_delivery (
    id BIGSERIAL PRIMARY KEY,
    announcement_id BIGINT NOT NULL REFERENCES announcement(id) ON DELETE CASCADE,
    customer_id BIGINT NOT NULL REFERENCES customer(id) ON DELETE CASCADE,
    telegram_message_id INTEGER,
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_announcement_delivery UNIQUE (announcement_id, customer_id)
);

-- Indexes for announcement_delivery
CREATE INDEX IF NOT EXISTS idx_announcement_delivery_announcement_id ON announcement_delivery (announcement_id);
CREATE INDEX IF NOT EXISTS idx_announcement_delivery_customer_id ON announcement_delivery (customer_id);