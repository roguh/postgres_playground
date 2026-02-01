package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

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

	fmt.Println("🚀 PostgreSQL Batch Operations\n")

	batchInserts(ctx, pool)
	batchUpdates(ctx, pool)
	copyFromDemo(ctx, pool)
	batchWithPipeline(ctx, pool)
}

func batchInserts(ctx context.Context, pool *database.Pool) {
	fmt.Println("=== Batch Inserts ===")

	// Method 1: Single INSERT with multiple VALUES (fast, simple)
	start := time.Now()

	// Build values
	values := make([]string, 100)
	args := make([]interface{}, 0, 400) // 4 args per row
	for i := 0; i < 100; i++ {
		values[i] = fmt.Sprintf("($%d, $%d, $%d, $%d)",
			i*4+1, i*4+2, i*4+3, i*4+4)
		args = append(args,
			fmt.Sprintf("Test Site %d", i),
			fmt.Sprintf("%d Test St", i),
			"Test City",
			"US")
	}

	query := fmt.Sprintf(`
		INSERT INTO sites (name, address, city, country)
		VALUES %s
		ON CONFLICT (name) DO NOTHING
		RETURNING id
	`, strings.Join(values, ","))

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Batch insert error: %v", err)
		return
	}

	count := 0
	for rows.Next() {
		count++
	}
	rows.Close()

	fmt.Printf("✓ Inserted %d sites in %v (multi-value)\n", count, time.Since(start))

	// Method 2: pgx.Batch (more flexible, supports different queries)
	start = time.Now()
	batch := &pgx.Batch{}

	for i := 0; i < 100; i++ {
		batch.Queue(`
			INSERT INTO assets (
				site_id, mac_address, serial_number, asset_type,
				manufacturer, model, status
			) VALUES (
				(SELECT id FROM sites ORDER BY RANDOM() LIMIT 1),
				$1, $2, $3, $4, $5, $6
			)`,
			fmt.Sprintf("00:00:00:00:%02x:%02x", i/256, i%256),
			fmt.Sprintf("BATCH%d", time.Now().Unix()+int64(i)),
			"sensor",
			"BatchCorp",
			"Model-X",
			"active")
	}

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	insertedCount := 0
	for i := 0; i < batch.Len(); i++ {
		_, err := br.Exec()
		if err == nil {
			insertedCount++
		}
	}

	fmt.Printf("✓ Inserted %d assets in %v (pgx.Batch)\n",
		insertedCount, time.Since(start))

	// Method 3: Prepared statement reuse
	start = time.Now()

	// Transaction for prepared statement
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("Begin tx error: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	_, err = tx.Prepare(ctx, "batch_update", `
		UPDATE assets
		SET last_seen = NOW(),
		    telemetry = telemetry || $2
		WHERE serial_number = $1
	`)
	if err != nil {
		log.Printf("Prepare error: %v", err)
		return
	}

	updateCount := 0
	for i := 0; i < 50; i++ {
		telemetry := fmt.Sprintf(`{"batch_update": %d, "timestamp": "%s"}`,
			i, time.Now().Format(time.RFC3339))

		_, err := tx.Exec(ctx, "batch_update",
			fmt.Sprintf("BATCH%d", time.Now().Unix()-int64(i)),
			telemetry)
		if err == nil {
			updateCount++
		}
	}

	tx.Commit(ctx)
	fmt.Printf("✓ Updated %d assets in %v (prepared statement)\n",
		updateCount, time.Since(start))
}

func batchUpdates(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== Batch Updates ===")

	// Method 1: UPDATE with unnest() for bulk updates
	start := time.Now()

	ids := make([]string, 20)
	statuses := make([]string, 20)

	// Get some asset IDs
	rows, err := pool.Query(ctx,
		"SELECT id FROM assets WHERE status = 'active' LIMIT 20")
	if err != nil {
		log.Printf("Query error: %v", err)
		return
	}

	i := 0
	for rows.Next() && i < 20 {
		rows.Scan(&ids[i])
		statuses[i] = "maintenance"
		i++
	}
	rows.Close()

	result, err := pool.Exec(ctx, `
		UPDATE assets a
		SET status = u.status,
		    updated_at = NOW()
		FROM (
			SELECT unnest($1::uuid[]) as id,
			       unnest($2::text[]) as status
		) u
		WHERE a.id = u.id
	`, ids[:i], statuses[:i])

	if err == nil {
		fmt.Printf("✓ Bulk updated %d assets in %v (unnest)\n",
			result.RowsAffected(), time.Since(start))
	}

	// Method 2: UPDATE with CASE for different values
	start = time.Now()

	caseQuery := `
		UPDATE assets
		SET telemetry = telemetry ||
			CASE serial_number
	`

	args := []interface{}{}
	whereClause := []string{}

	for j := 0; j < 10; j++ {
		serial := fmt.Sprintf("BATCH%d", time.Now().Unix()-int64(j*10))
		caseQuery += fmt.Sprintf(`
				WHEN $%d THEN $%d::jsonb`, len(args)+1, len(args)+2)
		args = append(args, serial,
			fmt.Sprintf(`{"case_update": %d, "metric": %d}`, j, j*10))
		whereClause = append(args, fmt.Sprintf("$%d", len(args)))
	}

	caseQuery += `
			END
		WHERE serial_number IN (` + strings.Join(whereClause[len(whereClause)-10:], ",") + ")"

	result, err = pool.Exec(ctx, caseQuery, args...)
	if err == nil {
		fmt.Printf("✓ Case-updated %d assets in %v\n",
			result.RowsAffected(), time.Since(start))
	}

	// Method 3: Temporary table for complex updates
	start = time.Now()

	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		// Create temp table
		_, err := tx.Exec(ctx, `
			CREATE TEMP TABLE batch_updates (
				site_name TEXT,
				new_metadata JSONB
			) ON COMMIT DROP
		`)
		if err != nil {
			return err
		}

		// Populate temp table
		batch := &pgx.Batch{}
		for i := 0; i < 30; i++ {
			batch.Queue(
				"INSERT INTO batch_updates VALUES ($1, $2)",
				fmt.Sprintf("Test Site %d", i),
				fmt.Sprintf(`{"batch_process": true, "update_num": %d}`, i))
		}

		br := tx.SendBatch(ctx, batch)
		br.Close()

		// Perform update join
		result, err := tx.Exec(ctx, `
			UPDATE sites s
			SET metadata = s.metadata || bu.new_metadata
			FROM batch_updates bu
			WHERE s.name = bu.site_name
		`)

		if err == nil {
			fmt.Printf("✓ Temp-table updated %d sites in %v\n",
				result.RowsAffected(), time.Since(start))
		}

		return err
	})

	if err != nil {
		log.Printf("Temp table update error: %v", err)
	}
}

func copyFromDemo(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== COPY FROM (Fastest Bulk Insert) ===")

	start := time.Now()

	// Prepare data
	type assetRow struct {
		siteID       string
		macAddress   string
		serialNumber string
		assetType    string
		manufacturer string
		model        string
		status       string
		config       string
		telemetry    string
	}

	// Get a site ID
	var siteID string
	pool.QueryRow(ctx, "SELECT id FROM sites LIMIT 1").Scan(&siteID)

	// Generate rows
	rows := make([]assetRow, 10000)
	for i := range rows {
		rows[i] = assetRow{
			siteID:       siteID,
			macAddress:   fmt.Sprintf("AA:BB:CC:%02X:%02X:%02X", i/65536, (i/256)%256, i%256),
			serialNumber: fmt.Sprintf("COPY%d%d", time.Now().Unix(), i),
			assetType:    "sensor",
			manufacturer: "CopyTest",
			model:        "CT-1000",
			status:       "active",
			config:       `{"copy_test": true}`,
			telemetry:    fmt.Sprintf(`{"batch": %d}`, i/1000),
		}
	}

	// Use CopyFrom
	copyCount, err := pool.CopyFrom(
		ctx,
		pgx.Identifier{"assets"},
		[]string{
			"site_id", "mac_address", "serial_number", "asset_type",
			"manufacturer", "model", "status", "config", "telemetry",
		},
		pgx.CopyFromSlice(len(rows), func(i int) ([]interface{}, error) {
			r := rows[i]
			return []interface{}{
				r.siteID, r.macAddress, r.serialNumber, r.assetType,
				r.manufacturer, r.model, r.status, r.config, r.telemetry,
			}, nil
		}),
	)

	if err != nil {
		log.Printf("CopyFrom error: %v", err)
	} else {
		fmt.Printf("✓ Inserted %d assets in %v (%.0f rows/sec)\n",
			copyCount, time.Since(start),
			float64(copyCount)/time.Since(start).Seconds())
	}

	// COPY with custom reader for streaming data
	fmt.Println("\n✓ Streaming COPY example:")

	// Clean up test data
	pool.Exec(ctx, "DELETE FROM assets WHERE manufacturer = 'CopyTest'")
}

func batchWithPipeline(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== Pipeline Mode (Maximum Throughput) ===")

	// Pipeline mode sends queries without waiting for results
	start := time.Now()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Printf("Acquire error: %v", err)
		return
	}
	defer conn.Release()

	// Start pipeline
	pipeline := conn.Conn().Pipeline()

	// Queue multiple queries
	results := make([]*pgx.Results, 100)
	for i := 0; i < 100; i++ {
		results[i] = pipeline.Query(ctx, `
			UPDATE assets
			SET last_seen = NOW(),
			    telemetry = telemetry || $1
			WHERE asset_type = $2
			LIMIT 10
		`, fmt.Sprintf(`{"pipeline": %d}`, i), "sensor")
	}

	// Execute pipeline
	err = pipeline.Sync(ctx)
	if err != nil {
		log.Printf("Pipeline sync error: %v", err)
		return
	}

	// Process results
	totalUpdated := int64(0)
	for _, res := range results {
		tag, err := res.Close()
		if err == nil {
			totalUpdated += tag.RowsAffected()
		}
	}

	err = pipeline.Close()
	if err == nil {
		fmt.Printf("✓ Pipeline updated %d rows in %v\n",
			totalUpdated, time.Since(start))
	}

	// Best practices summary
	fmt.Println("\n📋 Batch Operation Best Practices:")
	fmt.Println("  1. COPY FROM: Fastest for bulk inserts (100k+ rows/sec)")
	fmt.Println("  2. Multi-VALUE INSERT: Good for moderate batches (<1000 rows)")
	fmt.Println("  3. Pipeline mode: Best for many independent operations")
	fmt.Println("  4. Temp tables: Complex updates with joins")
	fmt.Println("  5. Always use transactions for consistency")
	fmt.Println("  6. Monitor pg_stat_statements for performance")
}
