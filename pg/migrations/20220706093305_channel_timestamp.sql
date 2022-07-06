-- +goose Up
-- +goose StatementBegin
ALTER TABLE channels ADD COLUMN prbody_timestamp TEXT NOT NULL DEFAULT '';
ALTER TABLE channels ALTER COLUMN prbody_timestamp DROP DEFAULT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE channels DROP COLUMN prbody_timestamp;
-- +goose StatementEnd
