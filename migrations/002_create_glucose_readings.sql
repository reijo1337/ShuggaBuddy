-- +goose Up
CREATE TABLE glucose_readings (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    value_mmol  NUMERIC(5,2) NOT NULL,
    source      TEXT NOT NULL DEFAULT 'manual',
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_glucose_readings_user_id_recorded_at
    ON glucose_readings (user_id, recorded_at DESC);

-- +goose Down
DROP TABLE IF EXISTS glucose_readings;
