CREATE TABLE IF NOT EXISTS orders
(
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    number      TEXT   NOT NULL UNIQUE,
    status      TEXT   NOT NULL          DEFAULT 'NEW', -- NEW, PROCESSING, PROCESSED, INVALID
    accrual     NUMERIC(10, 2)           DEFAULT 0,
    uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_orders_user_id ON orders (user_id);
CREATE INDEX idx_orders_status ON orders (status);