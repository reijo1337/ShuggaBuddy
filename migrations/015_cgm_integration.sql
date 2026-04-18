-- +goose Up
CREATE TABLE cgm_connections (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    provider        TEXT NOT NULL DEFAULT 'nightscout',
    base_url        TEXT NOT NULL,
    api_token       TEXT NOT NULL,
    last_synced_at  TIMESTAMPTZ,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, provider)
);

ALTER TABLE glucose_readings ADD COLUMN trend TEXT;

CREATE UNIQUE INDEX idx_glucose_readings_cgm_dedup
    ON glucose_readings (user_id, source, recorded_at)
    WHERE source = 'nightscout';

-- +goose Down
DROP INDEX IF EXISTS idx_glucose_readings_cgm_dedup;
ALTER TABLE glucose_readings DROP COLUMN IF EXISTS trend;
DROP TABLE IF EXISTS cgm_connections;
