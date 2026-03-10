-- +goose Up
CREATE TABLE users (
    id         BIGINT PRIMARY KEY,
    username   TEXT,
    first_name TEXT,
    units      TEXT NOT NULL DEFAULT 'mmol',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS users;
