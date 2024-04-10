-- +goose Down
ALTER TABLE observations
DROP COLUMN client_id;
