-- +goose Up
ALTER TABLE users ADD COLUMN carbs_per_unit NUMERIC(4,1) NOT NULL DEFAULT 12;

-- +goose Down
ALTER TABLE users DROP COLUMN carbs_per_unit;
