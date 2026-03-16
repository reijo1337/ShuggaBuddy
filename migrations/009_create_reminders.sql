-- +goose Up
CREATE TABLE reminders (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    activity_id BIGINT NOT NULL REFERENCES activity_entries(id),
    chat_id     BIGINT NOT NULL,
    fire_at     TIMESTAMPTZ NOT NULL,
    fired       BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reminders_pending
    ON reminders (fire_at) WHERE fired = false;

-- +goose Down
DROP TABLE IF EXISTS reminders;
