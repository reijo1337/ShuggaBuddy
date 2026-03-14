-- +goose Up
CREATE TABLE insulin_doses (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    dose_units   NUMERIC(6,2) NOT NULL,
    insulin_type TEXT NOT NULL,
    drug         TEXT NOT NULL DEFAULT '',
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_insulin_doses_user_id_recorded_at
    ON insulin_doses (user_id, recorded_at DESC);

-- +goose Down
DROP TABLE IF EXISTS insulin_doses;
