-- +goose Down
DROP TABLE IF EXISTS target_populations;

-- +goose Down
DROP INDEX IF EXISTS target_populations_site_id_idx;

-- +goose Down
DROP INDEX IF EXISTS target_populations_species_id_idx;
