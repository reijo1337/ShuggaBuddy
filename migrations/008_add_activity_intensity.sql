-- +goose Up
ALTER TABLE activity_entries ADD COLUMN intensity TEXT NOT NULL DEFAULT 'medium';

-- +goose Down
ALTER TABLE activity_entries DROP COLUMN intensity;
