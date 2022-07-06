-- +goose Up
-- +goose StatementBegin
ALTER TABLE channels ADD COLUMN prbody_timestamp TEXT NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE channels DROP COLUMN prbody_timestamp;
-- +goose StatementEnd
