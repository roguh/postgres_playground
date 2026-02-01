-- Drop triggers
DROP TRIGGER IF EXISTS update_assets_updated_at ON assets;
DROP TRIGGER IF EXISTS update_sites_updated_at ON sites;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_assets_last_seen;
DROP INDEX IF EXISTS idx_assets_telemetry;
DROP INDEX IF EXISTS idx_assets_config;
DROP INDEX IF EXISTS idx_assets_status;
DROP INDEX IF EXISTS idx_assets_mac;
DROP INDEX IF EXISTS idx_assets_site_id;

DROP INDEX IF EXISTS idx_sites_metadata;
DROP INDEX IF EXISTS idx_sites_coordinates;
DROP INDEX IF EXISTS idx_sites_country;

-- Drop tables
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS sites;

-- Drop extensions
DROP EXTENSION IF EXISTS "pg_stat_statements";
DROP EXTENSION IF EXISTS "uuid-ossp";
