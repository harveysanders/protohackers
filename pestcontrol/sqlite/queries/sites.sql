-- name: GetSite :one
SELECT
  *
FROM
  sites
WHERE
  id = ?
LIMIT
  1;

-- name: GetSiteTargetPopulations :many
SELECT
  target_populations.*,
  species.*
FROM
  target_populations
  JOIN species ON target_populations.species_id = species.id
WHERE
  site_id = ?;

-- name: CreateSite :one
INSERT INTO
  sites (id, created_at)
VALUES
  (?, ?) RETURNING *;

-- name: CreateSpecies :one
INSERT INTO
  species (name, created_at)
VALUES
  (?, ?) RETURNING *;

-- name: GetSpeciesByName :one
SELECT
  *
FROM
  species
WHERE
  name = ?;

-- name: CreateTargetPopulation :one
INSERT INTO
  target_populations (site_id, species_id, min, max, created_at)
VALUES
  (?, ?, ?, ?, ?) RETURNING *;
