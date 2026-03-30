-- +goose Up
CREATE TABLE notes (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    type       TEXT NOT NULL,
    wellbeing  TEXT,
    text       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notes_user_id_created_at ON notes(user_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_notes_user_id_created_at;
DROP TABLE IF EXISTS notes;
