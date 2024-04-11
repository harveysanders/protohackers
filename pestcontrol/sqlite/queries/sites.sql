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

-- name: GetSiteSpeciesTargetPopulation :one
SELECT
  target_populations.*,
  species.*
FROM
  target_populations
  JOIN species ON target_populations.species_id = species.id
WHERE
  site_id = ?
  AND species.name = ?;

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

-- name: CreateObservation :one
INSERT INTO
  observations (site_id, species_id, count, created_at, client_id)
VALUES
  (?, ?, ?, ?, ?) RETURNING *;

-- name: GetSiteObservationForSpecies :many
SELECT
  *
FROM
  observations
WHERE
  site_id = ?
  AND species_id = ?
ORDER BY
  created_at DESC;

-- name: CreatePolicy :one
INSERT INTO
  policies (id, population_id, action, created_at)
VALUES
  (?, ?, ?, ?) RETURNING *;

-- name: GetPolicyBySiteSpecies :one
SELECT
  *
FROM
  policies
  JOIN target_populations ON policies.population_id = target_populations.id
  JOIN species ON target_populations.species_id = species.id
WHERE
  target_populations.site_id = ?
  AND species.name = ?;

-- name: DeletePolicy :one
UPDATE policies
SET
  deleted_at = ?
WHERE
  id = ? RETURNING *;

-- name: GetObservationBySpecies :one
SELECT
  *
FROM
  observations
WHERE
  site_id = ?
  AND species_id = ?
ORDER BY
  created_at DESC;
