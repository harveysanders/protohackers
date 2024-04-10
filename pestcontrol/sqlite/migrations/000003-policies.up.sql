-- +goose Up
CREATE TABLE policies (
  id integer NOT NULL PRIMARY KEY,
  created_at text NOT NULL,
  deleted_at text,
  action integer NOT NULL,
  population_id integer NOT NULL
);
