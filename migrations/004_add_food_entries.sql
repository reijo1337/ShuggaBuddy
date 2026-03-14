-- +goose Up
CREATE TABLE food_entries (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    carbs_grams NUMERIC(6,1) NOT NULL,
    note        TEXT NOT NULL DEFAULT '',
    eaten_at    TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_food_entries_user_id_eaten_at
    ON food_entries (user_id, eaten_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_food_entries_user_id_eaten_at;
DROP TABLE IF EXISTS food_entries;
