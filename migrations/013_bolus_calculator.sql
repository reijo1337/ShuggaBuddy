-- +goose Up
ALTER TABLE users ADD COLUMN bolus_drug TEXT NOT NULL DEFAULT '';
ALTER TABLE insulin_doses ADD COLUMN source TEXT NOT NULL DEFAULT 'manual';

-- +goose Down
ALTER TABLE insulin_doses DROP COLUMN source;
ALTER TABLE users DROP COLUMN bolus_drug;
