package main

import (
	"context"
	"fmt"
	"log"

	"roguh.com/postgres_playground/pkg/database"
)

func main() {
	// Create context for all operations
	ctx := context.Background()

	// Connect to PostgreSQL
	pool, err := database.NewPool(ctx, database.DefaultConfig())
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer pool.Close()

	fmt.Println("🐘 Welcome to PostgreSQL Playground!")
	fmt.Println("===================================")

	// Test connection
	var result int
	err = pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		log.Fatal("Connection test failed:", err)
	}
	fmt.Printf("✓ Connected to PostgreSQL (1 = %d)\n", result)

	// Show pool statistics
	fmt.Printf("✓ %s\n", pool.Stats())

	// Count data
	var siteCount, assetCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM sites").Scan(&siteCount)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM assets").Scan(&assetCount)

	fmt.Printf("\n📊 Database Contents:\n")
	fmt.Printf("   Sites:  %d\n", siteCount)
	fmt.Printf("   Assets: %d\n", assetCount)

	if siteCount == 0 {
		fmt.Println("\n⚠️  No data found. Run 'make seed' to populate the database.")
		return
	}

	// Simple query example
	fmt.Println("\n📍 Sample Sites:")
	rows, err := pool.Query(ctx, `
		SELECT name, city, country
		FROM sites
		ORDER BY RANDOM()
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name, city, country string
		if err := rows.Scan(&name, &city, &country); err != nil {
			continue
		}
		fmt.Printf("   - %s (%s, %s)\n", name, city, country)
	}

	// JSON query example
	fmt.Println("\n🔧 Sample Asset Configurations:")
	rows, err = pool.Query(ctx, `
		SELECT
			serial_number,
			asset_type,
			config->>'ip' as ip_address
		FROM assets
		WHERE config ? 'ip'
		ORDER BY RANDOM()
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var serial, assetType, ip string
		if err := rows.Scan(&serial, &assetType, &ip); err != nil {
			continue
		}
		fmt.Printf("   - %s (%s): IP=%s\n", serial, assetType, ip)
	}

	fmt.Println("\n✨ Ready to explore! Try running:")
	fmt.Println("   go run examples/01_basic_queries.go")
	fmt.Println("   go run examples/02_json_queries.go")
	fmt.Println("   go run examples/03_batch_operations.go")
	fmt.Println("   go run examples/04_advanced_patterns.go")
}
