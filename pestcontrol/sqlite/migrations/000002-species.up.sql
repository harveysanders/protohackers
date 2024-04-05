-- +goose Up
CREATE TABLE species (
  id integer PRIMARY KEY AUTOINCREMENT,
  created_at text NOT NULL,
  name varchar(255) UNIQUE NOT NULL
);
