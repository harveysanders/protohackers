-- +goose Up
CREATE TABLE target_populations (
  id integer NOT NULL PRIMARY KEY,
  created_at text NOT NULL,
  site_id integer NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
  species_id integer NOT NULL REFERENCES species (id) ON DELETE CASCADE,
  min integer NOT NULL,
  max integer NOT NULL,
  UNIQUE (site_id, species_id)
);

-- +goose Up
CREATE INDEX target_populations_site_id_idx ON target_populations (site_id);

-- +goose Up
CREATE INDEX target_populations_species_id_idx ON target_populations (species_id);
