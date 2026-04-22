-- +goose Up
ALTER TABLE cgm_connections ADD COLUMN region TEXT;

CREATE UNIQUE INDEX idx_glucose_readings_llu_dedup
    ON glucose_readings (user_id, source, recorded_at)
    WHERE source = 'librelinkup';

-- +goose Down
DROP INDEX IF EXISTS idx_glucose_readings_llu_dedup;
ALTER TABLE cgm_connections DROP COLUMN IF EXISTS region;
