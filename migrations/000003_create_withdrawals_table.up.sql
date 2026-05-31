CREATE TABLE IF NOT EXISTS withdrawals
(
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    order_number TEXT           NOT NULL UNIQUE,
    amount       NUMERIC(10, 2) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_withdrawals_user_id ON withdrawals (user_id);