-- +goose Up
ALTER TABLE users ADD COLUMN basal_dose FLOAT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN advisor_interval_days INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN advisor_last_sent_at TIMESTAMP;

-- +goose Down
ALTER TABLE users DROP COLUMN advisor_last_sent_at;
ALTER TABLE users DROP COLUMN advisor_interval_days;
ALTER TABLE users DROP COLUMN basal_dose;
