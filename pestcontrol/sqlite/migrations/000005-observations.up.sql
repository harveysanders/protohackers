-- +goose Up
CREATE TABLE observations (
  id integer PRIMARY KEY,
  created_at text NOT NULL,
  site_id integer NOT NULL,
  species_id integer NOT NULL,
  count integer NOT NULL
);
