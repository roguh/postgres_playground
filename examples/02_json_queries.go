package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"roguh.com/postgres_playground/pkg/database"
)

func main() {
	ctx := context.Background()
	pool, err := database.NewPool(ctx, database.DefaultConfig())
	if err != nil {
		log.Fatal("Failed to create pool:", err)
	}
	defer pool.Close()

	fmt.Println("🔍 PostgreSQL JSON/JSONB Examples\n")

	basicJSONQueries(ctx, pool)
	jsonPathQueries(ctx, pool)
	jsonAggregation(ctx, pool)
	jsonIndexing(ctx, pool)
}

func basicJSONQueries(ctx context.Context, pool *database.Pool) {
	fmt.Println("=== Basic JSON Queries ===")

	// 1. Extract simple value
	var managerName string
	err := pool.QueryRow(ctx, `
		SELECT metadata->>'manager'
		FROM sites
		WHERE metadata ? 'manager'
		LIMIT 1
	`).Scan(&managerName)

	if err == nil {
		fmt.Printf("✓ Found manager: %s\n", managerName)
	}

	// 2. Check JSON contains key
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM sites
		WHERE metadata ? 'compliance'
	`).Scan(&count)

	if err == nil {
		fmt.Printf("✓ Sites with compliance data: %d\n", count)
	}

	// 3. Query nested JSON
	fmt.Println("\n✓ Sites with certifications:")
	rows, err := pool.Query(ctx, `
		SELECT
			name,
			metadata->'compliance'->'certifications' as certs
		FROM sites
		WHERE metadata->'compliance'->'certifications' IS NOT NULL
			AND jsonb_array_length(metadata->'compliance'->'certifications') > 0
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var certs json.RawMessage
		if err := rows.Scan(&name, &certs); err != nil {
			continue
		}
		fmt.Printf("  - %s: %s\n", name, string(certs))
	}

	// 4. JSON operators
	fmt.Println("\n✓ Different JSON operators:")

	// @> contains
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM assets
		WHERE config @> '{"network": {"interfaces": [{"name": "eth0"}]}}'
	`).Scan(&count)
	if err == nil {
		fmt.Printf("  - Assets with eth0 interface: %d\n", count)
	}

	// <@ is contained by
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM assets
		WHERE '{"status": "up"}' <@ config
	`).Scan(&count)
	if err == nil {
		fmt.Printf("  - Assets containing status=up: %d\n", count)
	}
}

func jsonPathQueries(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== JSON Path Queries ===")

	// 1. Extract with #> path operator
	fmt.Println("✓ Extract nested values with paths:")
	rows, err := pool.Query(ctx, `
		SELECT
			serial_number,
			config #> '{network,interfaces,0,ip}' as primary_ip,
			config #> '{settings,temp_threshold}' as temp_threshold
		FROM assets
		WHERE config #> '{network,interfaces,0,ip}' IS NOT NULL
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var serial string
		var ip, threshold json.RawMessage
		if err := rows.Scan(&serial, &ip, &threshold); err != nil {
			continue
		}
		fmt.Printf("  - %s: IP=%s, Threshold=%s\n", serial, string(ip), string(threshold))
	}

	// 2. Text extraction with #>>
	fmt.Println("\n✓ Extract as text with #>>:")
	var firmwareVersion string
	err = pool.QueryRow(ctx, `
		SELECT config #>> '{firmware,current}'
		FROM assets
		WHERE config #>> '{firmware,current}' IS NOT NULL
		LIMIT 1
	`).Scan(&firmwareVersion)

	if err == nil {
		fmt.Printf("  - Firmware version: %s\n", firmwareVersion)
	}

	// 3. Complex path queries
	fmt.Println("\n✓ Assets with high CPU readings:")
	rows, err = pool.Query(ctx, `
		WITH cpu_readings AS (
			SELECT
				serial_number,
				jsonb_array_elements(telemetry->'readings') as reading
			FROM assets
			WHERE telemetry->'readings' IS NOT NULL
		)
		SELECT
			serial_number,
			(reading->>'cpu')::float as cpu_usage,
			reading->>'ts' as timestamp
		FROM cpu_readings
		WHERE (reading->>'cpu')::float > 80
		ORDER BY cpu_usage DESC
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var serial, timestamp string
		var cpu float64
		if err := rows.Scan(&serial, &cpu, &timestamp); err != nil {
			continue
		}
		fmt.Printf("  - %s: %.1f%% at %s\n", serial, cpu, timestamp)
	}
}

func jsonAggregation(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== JSON Aggregation ===")

	// 1. Build JSON from query results
	var result json.RawMessage
	err := pool.QueryRow(ctx, `
		SELECT jsonb_build_object(
			'total_sites', COUNT(DISTINCT s.id),
			'total_assets', COUNT(DISTINCT a.id),
			'countries', jsonb_agg(DISTINCT s.country),
			'asset_types', jsonb_agg(DISTINCT a.asset_type)
		)
		FROM sites s
		JOIN assets a ON a.site_id = s.id
	`).Scan(&result)

	if err == nil {
		fmt.Printf("✓ System overview: %s\n", string(result))
	}

	// 2. Aggregate JSON arrays
	fmt.Println("\n✓ Aggregate certifications by country:")
	rows, err := pool.Query(ctx, `
		SELECT
			country,
			jsonb_agg(DISTINCT cert) as all_certs
		FROM sites,
			jsonb_array_elements_text(metadata->'compliance'->'certifications') as cert
		WHERE metadata->'compliance'->'certifications' IS NOT NULL
		GROUP BY country
		ORDER BY country
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var country string
		var certs json.RawMessage
		if err := rows.Scan(&country, &certs); err != nil {
			continue
		}
		fmt.Printf("  - %s: %s\n", country, string(certs))
	}

	// 3. JSON object aggregation
	fmt.Println("\n✓ Asset summary by type:")
	rows, err = pool.Query(ctx, `
		SELECT
			asset_type,
			jsonb_build_object(
				'count', COUNT(*),
				'active', COUNT(*) FILTER (WHERE status = 'active'),
				'avg_uptime_hours',
					AVG(EXTRACT(EPOCH FROM (NOW() - last_seen))/3600)::int,
				'manufacturers', jsonb_agg(DISTINCT manufacturer)
			) as summary
		FROM assets
		GROUP BY asset_type
		ORDER BY COUNT(*) DESC
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetType string
		var summary json.RawMessage
		if err := rows.Scan(&assetType, &summary); err != nil {
			continue
		}
		fmt.Printf("  - %-10s: %s\n", assetType, string(summary))
	}
}

func jsonIndexing(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== JSON Index Usage ===")

	// Show query plans to demonstrate index usage
	fmt.Println("✓ Query plans showing GIN index usage:")

	// 1. GIN index on entire JSONB column
	var plan string
	row := pool.QueryRow(ctx, `
		EXPLAIN (FORMAT TEXT, ANALYZE false)
		SELECT COUNT(*)
		FROM assets
		WHERE config @> '{"network": {"interfaces": [{"name": "eth0"}]}}'
	`)
	if err := row.Scan(&plan); err == nil {
		fmt.Printf("\n  Full GIN index (@> operator):\n%s\n", plan)
	}

	// 2. GIN index with jsonb_path_ops
	row = pool.QueryRow(ctx, `
		EXPLAIN (FORMAT TEXT, ANALYZE false)
		SELECT COUNT(*)
		FROM sites
		WHERE metadata @> '{"facility": {"type": "warehouse"}}'
	`)
	if err := row.Scan(&plan); err == nil {
		fmt.Printf("\n  GIN with path ops:\n%s\n", plan)
	}

	// 3. Expression index on specific path
	// First create the index
	_, err := pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_assets_firmware_version
		ON assets((config->'firmware'->>'current'))
	`)
	if err != nil {
		log.Printf("Error creating index: %v", err)
	}

	row = pool.QueryRow(ctx, `
		EXPLAIN (FORMAT TEXT, ANALYZE false)
		SELECT COUNT(*)
		FROM assets
		WHERE config->'firmware'->>'current' = '2.1.0'
	`)
	if err := row.Scan(&plan); err == nil {
		fmt.Printf("\n  Expression index on specific path:\n%s\n", plan)
	}

	// 4. Partial index for JSON queries
	_, err = pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_sites_with_compliance
		ON sites(id)
		WHERE metadata ? 'compliance'
	`)
	if err != nil {
		log.Printf("Error creating partial index: %v", err)
	}

	// Real-world JSON query patterns
	fmt.Println("\n✓ Practical JSON query examples:")

	// Find assets with specific config patterns
	err = pool.QueryRow(ctx, `
		WITH feature_stats AS (
			SELECT
				key as feature,
				COUNT(*) as enabled_count
			FROM assets,
				jsonb_each(config->'features') as f(key, value)
			WHERE value->>'enabled' = 'true'
			GROUP BY key
		)
		SELECT jsonb_object_agg(feature, enabled_count)
		FROM feature_stats
	`).Scan(&result)

	if err == nil {
		fmt.Printf("  - Enabled features across fleet: %s\n", string(result))
	}

	// Complex filtering with JSON
	fmt.Println("\n✓ Assets matching complex criteria:")
	rows, err := pool.Query(ctx, `
		SELECT
			serial_number,
			asset_type,
			config->'settings'->>'power_mode' as power_mode,
			telemetry->'metrics'->'cpu'->>'value' as cpu
		FROM assets
		WHERE
			-- Has specific config structure
			config ? 'settings'
			-- Power mode is performance
			AND config->'settings'->>'power_mode' = 'performance'
			-- Has CPU telemetry
			AND telemetry->'metrics'->'cpu' IS NOT NULL
			-- CPU usage is high
			AND (telemetry->'metrics'->'cpu'->>'value')::float > 70
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var serial, assetType, powerMode, cpu string
		if err := rows.Scan(&serial, &assetType, &powerMode, &cpu); err != nil {
			continue
		}
		fmt.Printf("  - %s (%s): power=%s, cpu=%s%%\n",
			serial, assetType, powerMode, cpu)
	}

	// Transaction safety with JSON updates
	fmt.Println("\n✓ Safe JSON updates in transaction:")
	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		// Update JSON atomically
		_, err := tx.Exec(ctx, `
			UPDATE assets
			SET
				config = jsonb_set(
					config,
					'{settings,auto_update}',
					'true'
				)
			WHERE
				asset_type = 'router'
				AND config->'settings' IS NOT NULL
				AND NOT (config->'settings' ? 'auto_update')
			LIMIT 10
		`)
		return err
	})

	if err == nil {
		fmt.Println("  - Successfully enabled auto_update for routers")
	}
}
