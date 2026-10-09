CREATE TABLE IF NOT EXISTS users
(
    id            SERIAL PRIMARY KEY,
    login         VARCHAR(256) UNIQUE      NOT NUll,
    password_hash VARCHAR(1024)            NOT NULL,
    created_at    TIMESTAMP WITH TIME ZONE NOT NULL
);