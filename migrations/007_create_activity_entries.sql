-- +goose Up
CREATE TABLE activity_entries (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id),
    activity_type TEXT NOT NULL,
    custom_type   TEXT NOT NULL DEFAULT '',
    duration_min  INT NOT NULL,
    recorded_at   TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_activity_entries_user_recorded
    ON activity_entries (user_id, recorded_at DESC);

-- +goose Down
DROP TABLE IF EXISTS activity_entries;
