package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"roguh.com/postgres_playground/pkg/database"
)

func main() {
	ctx := context.Background()
	pool, err := database.NewPool(ctx, database.DefaultConfig())
	if err != nil {
		log.Fatal("Failed to create pool:", err)
	}
	defer pool.Close()

	// Demo each query pattern
	fmt.Println("🔍 PostgreSQL Query Examples\n")

	basicQueries(ctx, pool)
	joinQueries(ctx, pool)
	aggregateQueries(ctx, pool)
}

func basicQueries(ctx context.Context, pool *database.Pool) {
	fmt.Println("=== Basic Queries ===")

	// 1. Simple SELECT with prepared statement
	var siteName, city, country string
	err := pool.QueryRow(ctx,
		"SELECT name, city, country FROM sites WHERE country = $1 LIMIT 1",
		"US").Scan(&siteName, &city, &country)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("✓ Found site: %s in %s, %s\n", siteName, city, country)

	// 2. Multiple rows with proper resource cleanup
	rows, err := pool.Query(ctx,
		"SELECT serial_number, asset_type, status FROM assets WHERE status = $1 LIMIT 5",
		"active")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	fmt.Println("\n✓ Active assets:")
	for rows.Next() {
		var serial, assetType, status string
		if err := rows.Scan(&serial, &assetType, &status); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		fmt.Printf("  - %s (%s) - %s\n", serial, assetType, status)
	}

	// Always check rows.Err() after iteration
	if err := rows.Err(); err != nil {
		log.Printf("Row error: %v", err)
	}
}

func joinQueries(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== JOIN Queries ===")

	// 1. Basic INNER JOIN
	query := `
		SELECT
			s.name as site_name,
			s.country,
			COUNT(a.id) as asset_count,
			COUNT(CASE WHEN a.status = 'active' THEN 1 END) as active_count
		FROM sites s
		INNER JOIN assets a ON a.site_id = s.id
		GROUP BY s.id, s.name, s.country
		HAVING COUNT(a.id) > 50
		ORDER BY asset_count DESC
		LIMIT 5
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	fmt.Println("✓ Sites with most assets:")
	for rows.Next() {
		var siteName, country string
		var assetCount, activeCount int
		if err := rows.Scan(&siteName, &country, &assetCount, &activeCount); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		fmt.Printf("  - %s (%s): %d assets (%d active)\n",
			siteName, country, assetCount, activeCount)
	}

	// 2. LEFT JOIN to find sites without assets
	var sitesWithoutAssets int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM sites s
		LEFT JOIN assets a ON a.site_id = s.id
		WHERE a.id IS NULL
	`).Scan(&sitesWithoutAssets)
	if err == nil {
		fmt.Printf("\n✓ Sites without assets: %d\n", sitesWithoutAssets)
	}

	// 3. Complex JOIN with subquery
	complexQuery := `
		WITH stale_assets AS (
			SELECT
				site_id,
				COUNT(*) as stale_count
			FROM assets
			WHERE last_seen < NOW() - INTERVAL '24 hours'
			GROUP BY site_id
		)
		SELECT
			s.name,
			s.city,
			COALESCE(sa.stale_count, 0) as stale_assets
		FROM sites s
		LEFT JOIN stale_assets sa ON sa.site_id = s.id
		WHERE sa.stale_count > 0
		ORDER BY sa.stale_count DESC
		LIMIT 3
	`

	rows, err = pool.Query(ctx, complexQuery)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	fmt.Println("\n✓ Sites with stale assets:")
	for rows.Next() {
		var name, city string
		var staleCount int
		if err := rows.Scan(&name, &city, &staleCount); err != nil {
			continue
		}
		fmt.Printf("  - %s (%s): %d stale assets\n", name, city, staleCount)
	}
}

func aggregateQueries(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== Aggregate Queries ===")

	// 1. Window functions
	query := `
		WITH ranked_assets AS (
			SELECT
				a.serial_number,
				a.asset_type,
				s.country,
				a.last_seen,
				ROW_NUMBER() OVER (PARTITION BY s.country ORDER BY a.last_seen DESC) as rn,
				COUNT(*) OVER (PARTITION BY s.country) as country_total
			FROM assets a
			JOIN sites s ON s.id = a.site_id
		)
		SELECT serial_number, asset_type, country, last_seen, country_total
		FROM ranked_assets
		WHERE rn <= 2
		ORDER BY country, rn
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	fmt.Println("✓ Most recently seen assets by country:")
	var lastCountry string
	for rows.Next() {
		var serial, assetType, country string
		var lastSeen time.Time
		var countryTotal int
		if err := rows.Scan(&serial, &assetType, &country, &lastSeen, &countryTotal); err != nil {
			continue
		}
		if country != lastCountry {
			fmt.Printf("\n  %s (total: %d assets):\n", country, countryTotal)
			lastCountry = country
		}
		fmt.Printf("    - %s (%s) - seen %s ago\n",
			serial, assetType, timeSince(lastSeen))
	}

	// 2. Rollup aggregation
	rollupQuery := `
		SELECT
			COALESCE(country, 'ALL COUNTRIES') as country,
			COALESCE(asset_type, 'All Types') as asset_type,
			COUNT(*) as count,
			AVG(EXTRACT(EPOCH FROM (NOW() - last_seen))/3600)::INT as avg_hours_since_seen
		FROM assets a
		JOIN sites s ON s.id = a.site_id
		GROUP BY ROLLUP(country, asset_type)
		HAVING COUNT(*) > 100
		ORDER BY country NULLS LAST, asset_type NULLS LAST
		LIMIT 10
	`

	fmt.Println("\n✓ Asset distribution (ROLLUP):")
	rows, err = pool.Query(ctx, rollupQuery)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var country, assetType string
		var count, avgHours int
		if err := rows.Scan(&country, &assetType, &count, &avgHours); err != nil {
			continue
		}
		fmt.Printf("  - %-15s %-12s: %6d assets (avg %dh since seen)\n",
			country, assetType, count, avgHours)
	}
}

func timeSince(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.0fh", d.Hours())
	}
	return fmt.Sprintf("%.0fd", d.Hours()/24)
}
