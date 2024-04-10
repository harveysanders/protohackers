-- +goose Up
ALTER TABLE observations
ADD COLUMN client_id integer NOT NULL;
