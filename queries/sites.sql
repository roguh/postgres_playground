-- name: CreateSite :one
INSERT INTO sites (
    name, address, city, country, coordinates, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetSite :one
SELECT * FROM sites
WHERE id = $1 LIMIT 1;

-- name: GetSiteWithAssets :many
SELECT
    s.id as site_id,
    s.name as site_name,
    s.address,
    s.city,
    s.country,
    s.metadata as site_metadata,
    a.id as asset_id,
    a.mac_address,
    a.serial_number,
    a.asset_type,
    a.status,
    a.config,
    a.telemetry,
    a.last_seen
FROM sites s
LEFT JOIN assets a ON a.site_id = s.id
WHERE s.id = $1
ORDER BY a.last_seen DESC;

-- name: ListSites :many
SELECT * FROM sites
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateSiteMetadata :one
UPDATE sites
SET metadata = metadata || $2
WHERE id = $1
RETURNING *;

-- name: SearchSitesByMetadata :many
SELECT * FROM sites
WHERE metadata @> $1
ORDER BY name;

-- name: FindSitesByCountry :many
SELECT * FROM sites
WHERE country = $1
ORDER BY city, name;

-- name: FindNearestSites :many
SELECT
    id,
    name,
    address,
    city,
    country,
    coordinates,
    metadata,
    coordinates <-> point($1, $2) AS distance
FROM sites
WHERE coordinates IS NOT NULL
ORDER BY coordinates <-> point($1, $2)
LIMIT $3;

-- name: DeleteSite :exec
DELETE FROM sites
WHERE id = $1;
