-- name: CreateAsset :one
INSERT INTO assets (
    site_id, mac_address, serial_number, asset_type,
    manufacturer, model, firmware_version, status, config, telemetry
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetAsset :one
SELECT * FROM assets
WHERE id = $1 LIMIT 1;

-- name: GetAssetBySerial :one
SELECT * FROM assets
WHERE serial_number = $1 LIMIT 1;

-- name: GetAssetsByMac :many
SELECT * FROM assets
WHERE mac_address = $1
ORDER BY created_at DESC;

-- name: ListAssetsBySite :many
SELECT * FROM assets
WHERE site_id = $1
ORDER BY last_seen DESC
LIMIT $2 OFFSET $3;

-- name: UpdateAssetStatus :one
UPDATE assets
SET status = $2, last_seen = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateAssetTelemetry :one
UPDATE assets
SET
    telemetry = telemetry || $2,
    last_seen = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateAssetConfig :one
UPDATE assets
SET config = $2
WHERE id = $1
RETURNING *;

-- name: SearchAssetsByConfig :many
SELECT * FROM assets
WHERE config @> $1
ORDER BY last_seen DESC;

-- name: FindStaleAssets :many
SELECT
    a.*,
    s.name as site_name,
    EXTRACT(EPOCH FROM (NOW() - a.last_seen))::INT as seconds_since_seen
FROM assets a
JOIN sites s ON s.id = a.site_id
WHERE a.last_seen < NOW() - INTERVAL '1 hour' * $1
ORDER BY a.last_seen ASC;

-- name: GetAssetTelemetryValue :one
SELECT telemetry #>> $2 as value
FROM assets
WHERE id = $1;

-- name: FindAssetsByTelemetryRange :many
SELECT * FROM assets
WHERE (telemetry->$1->>'value')::float BETWEEN $2 AND $3
ORDER BY (telemetry->$1->>'value')::float DESC;

-- name: BulkUpdateAssetStatus :exec
UPDATE assets
SET status = $2, last_seen = NOW()
WHERE id = ANY($1::uuid[]);

-- name: GetAssetsWithComplexFilter :many
SELECT
    a.*,
    s.name as site_name,
    s.country as site_country
FROM assets a
JOIN sites s ON s.id = a.site_id
WHERE
    ($1::text IS NULL OR a.asset_type = $1)
    AND ($2::text IS NULL OR a.status = $2)
    AND ($3::text IS NULL OR s.country = $3)
    AND ($4::jsonb IS NULL OR a.config @> $4)
ORDER BY a.last_seen DESC
LIMIT $5 OFFSET $6;

-- name: DeleteAsset :exec
DELETE FROM assets
WHERE id = $1;
