-- +goose Up
ALTER TABLE users
    ADD COLUMN target_min_mmol FLOAT NOT NULL DEFAULT 3.9,
    ADD COLUMN target_max_mmol FLOAT NOT NULL DEFAULT 10.0,
    ADD COLUMN basal_drug      TEXT NOT NULL DEFAULT '',
    ADD COLUMN basal_time      TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users
    DROP COLUMN target_min_mmol,
    DROP COLUMN target_max_mmol,
    DROP COLUMN basal_drug,
    DROP COLUMN basal_time;
