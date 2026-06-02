ALTER TABLE users
    ADD COLUMN current_balance NUMERIC(10, 2) NOT NULL DEFAULT 0;
ALTER TABLE users
    ADD COLUMN total_withdrawn NUMERIC(10, 2) NOT NULL DEFAULT 0;

CREATE INDEX idx_orders_pending ON orders (status, uploaded_at) WHERE status IN ('NEW', 'PROCESSING');

CREATE INDEX idx_withdrawals_user_processed ON withdrawals(user_id, processed_at DESC);