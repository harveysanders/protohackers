-- +goose Up
ALTER TABLE policies
ADD CONSTRAINT policies_unique_constraint UNIQUE (site_id, population_id);
