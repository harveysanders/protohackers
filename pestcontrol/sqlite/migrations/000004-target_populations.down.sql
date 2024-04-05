-- +goose Down
DROP TABLE target_populations;

-- +goose Down
DROP INDEX target_populations_site_id_idx;

-- +goose Down
DROP INDEX target_populations_species_id_idx;
