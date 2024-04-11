-- +goose Down
ALTER TABLE policies
DROP CONSTRAINT policies_unique_constraint;
