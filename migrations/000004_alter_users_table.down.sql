ALTER TABLE users
    DROP COLUMN current_balance;
ALTER TABLE users
    DROP COLUMN total_withdrawn;

DROP INDEX IF EXISTS idx_orders_pending;

DROP INDEX IF EXISTS idx_withdrawals_user_processed;