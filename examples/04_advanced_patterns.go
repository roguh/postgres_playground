package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"roguh.com/postgres_playground/pkg/database"
)

func main() {
	ctx := context.Background()
	pool, err := database.NewPool(ctx, database.DefaultConfig())
	if err != nil {
		log.Fatal("Failed to create pool:", err)
	}
	defer pool.Close()

	fmt.Println("🚀 PostgreSQL Advanced Patterns\n")

	partitioningDemo(ctx, pool)
	listenNotifyDemo(ctx, pool)
	advisoryLocksDemo(ctx, pool)
	ctasAndMaterializedViews(ctx, pool)
	queryOptimization(ctx, pool)
}

func partitioningDemo(ctx context.Context, pool *database.Pool) {
	fmt.Println("=== Partitioning for Scale ===")

	// Create partitioned table for telemetry data
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		// Create parent table
		_, err := tx.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS telemetry_data (
				asset_id UUID NOT NULL,
				timestamp TIMESTAMPTZ NOT NULL,
				metrics JSONB NOT NULL,
				PRIMARY KEY (asset_id, timestamp)
			) PARTITION BY RANGE (timestamp)
		`)
		if err != nil {
			return err
		}

		// Create monthly partitions
		now := time.Now()
		for i := 0; i < 3; i++ {
			month := now.AddDate(0, -i, 0)
			start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 1, 0)

			partName := fmt.Sprintf("telemetry_data_%s", start.Format("2006_01"))
			_, err = tx.Exec(ctx, fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s
				PARTITION OF telemetry_data
				FOR VALUES FROM ('%s') TO ('%s')
			`, partName, start.Format(time.RFC3339), end.Format(time.RFC3339)))

			if err != nil {
				// Partition might already exist
				continue
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Partitioning setup error: %v", err)
		return
	}

	fmt.Println("✓ Created partitioned telemetry table")

	// Insert data across partitions
	batch := &pgx.Batch{}
	baseTime := time.Now().AddDate(0, -2, 0)

	for i := 0; i < 100; i++ {
		timestamp := baseTime.Add(time.Duration(i) * 24 * time.Hour)
		batch.Queue(`
			INSERT INTO telemetry_data (asset_id, timestamp, metrics)
			VALUES (
				(SELECT id FROM assets LIMIT 1 OFFSET $1),
				$2,
				$3
			)
			ON CONFLICT DO NOTHING
		`, i%10, timestamp, fmt.Sprintf(`{"day": %d, "value": %d}`, i, i*10))
	}

	br := pool.SendBatch(ctx, batch)
	br.Close()

	// Query partition info
	var partitionInfo []struct {
		tableName string
		rowCount  int64
	}

	rows, err := pool.Query(ctx, `
		SELECT
			schemaname || '.' || tablename as table_name,
			n_live_tup as row_count
		FROM pg_stat_user_tables
		WHERE tablename LIKE 'telemetry_data_%'
		ORDER BY tablename
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var info struct {
				tableName string
				rowCount  int64
			}
			rows.Scan(&info.tableName, &info.rowCount)
			partitionInfo = append(partitionInfo, info)
		}
	}

	fmt.Println("\n✓ Partition statistics:")
	for _, info := range partitionInfo {
		fmt.Printf("  - %s: %d rows\n", info.tableName, info.rowCount)
	}

	// Clean up
	pool.Exec(ctx, "DROP TABLE IF EXISTS telemetry_data CASCADE")
}

func listenNotifyDemo(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== LISTEN/NOTIFY for Real-time Events ===")

	// Create notification trigger
	_, err := pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION notify_asset_change()
		RETURNS trigger AS $$
		BEGIN
			PERFORM pg_notify(
				'asset_updates',
				json_build_object(
					'action', TG_OP,
					'asset_id', NEW.id,
					'serial', NEW.serial_number,
					'status', NEW.status,
					'timestamp', NOW()
				)::text
			);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
	`)
	if err != nil {
		log.Printf("Create function error: %v", err)
		return
	}

	// Attach trigger
	pool.Exec(ctx, `
		DROP TRIGGER IF EXISTS asset_update_notify ON assets;
		CREATE TRIGGER asset_update_notify
		AFTER INSERT OR UPDATE ON assets
		FOR EACH ROW EXECUTE FUNCTION notify_asset_change()
	`)

	// Start listener
	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Printf("Acquire error: %v", err)
		return
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN asset_updates")
	if err != nil {
		log.Printf("Listen error: %v", err)
		return
	}

	fmt.Println("✓ Listening for asset updates...")

	// Goroutine to receive notifications
	notifyChan := make(chan *pgconn.Notification, 5)
	go func() {
		for n := range notifyChan {
			fmt.Printf("  📢 Received: %s\n", n.Payload)
		}
	}()

	// Listen for notifications
	go func() {
		for i := 0; i < 3; i++ {
			notification, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				return
			}
			notifyChan <- notification
		}
		close(notifyChan)
	}()

	// Trigger some updates
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 3; i++ {
		_, err = pool.Exec(ctx, `
			UPDATE assets
			SET status = $1, last_seen = NOW()
			WHERE asset_type = 'sensor'
			LIMIT 1
		`, []string{"active", "maintenance", "offline"}[i%3])

		time.Sleep(200 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)

	// Clean up
	pool.Exec(ctx, "DROP TRIGGER IF EXISTS asset_update_notify ON assets")
	pool.Exec(ctx, "DROP FUNCTION IF EXISTS notify_asset_change()")
}

func advisoryLocksDemo(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== Advisory Locks for Distributed Coordination ===")

	// Demonstrate exclusive advisory locks
	var wg sync.WaitGroup
	results := make(chan string, 5)

	// Simulate multiple workers trying to process same resource
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			conn, err := pool.Acquire(ctx)
			if err != nil {
				results <- fmt.Sprintf("Worker %d: failed to acquire connection", workerID)
				return
			}
			defer conn.Release()

			// Try to get advisory lock (using a hash of resource ID)
			lockID := int64(12345) // Simulated resource ID

			var acquired bool
			err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
			if err != nil || !acquired {
				results <- fmt.Sprintf("Worker %d: couldn't acquire lock", workerID)
				return
			}

			// Do "work" with the locked resource
			results <- fmt.Sprintf("Worker %d: got lock, processing...", workerID)
			time.Sleep(200 * time.Millisecond)

			// Release lock
			conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID)
			results <- fmt.Sprintf("Worker %d: released lock", workerID)
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	fmt.Println("✓ Advisory lock results:")
	for result := range results {
		fmt.Printf("  - %s\n", result)
	}

	// Show lock status query
	fmt.Println("\n✓ Query to monitor advisory locks:")
	fmt.Println(`  SELECT pid, locktype, mode, granted
  FROM pg_locks
  WHERE locktype = 'advisory'`)
}

func ctasAndMaterializedViews(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== CTAS and Materialized Views ===")

	// Create summary table with CTAS
	start := time.Now()
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS site_summary AS
		SELECT
			s.id,
			s.name,
			s.country,
			COUNT(DISTINCT a.id) as asset_count,
			COUNT(DISTINCT a.asset_type) as asset_types,
			MAX(a.last_seen) as last_activity,
			jsonb_build_object(
				'total_active', COUNT(*) FILTER (WHERE a.status = 'active'),
				'total_offline', COUNT(*) FILTER (WHERE a.status = 'offline'),
				'types', jsonb_agg(DISTINCT a.asset_type)
			) as stats
		FROM sites s
		LEFT JOIN assets a ON a.site_id = s.id
		GROUP BY s.id, s.name, s.country
	`)

	if err == nil {
		fmt.Printf("✓ Created summary table with CTAS in %v\n", time.Since(start))
	}

	// Create materialized view for complex aggregations
	start = time.Now()
	_, err = pool.Exec(ctx, `
		CREATE MATERIALIZED VIEW IF NOT EXISTS mv_asset_telemetry_stats AS
		WITH latest_telemetry AS (
			SELECT DISTINCT ON (id)
				id,
				telemetry,
				last_seen
			FROM assets
			ORDER BY id, last_seen DESC
		)
		SELECT
			asset_type,
			COUNT(*) as total_assets,
			AVG((telemetry->'metrics'->'cpu'->>'value')::float) as avg_cpu,
			MAX((telemetry->'metrics'->'cpu'->>'value')::float) as max_cpu,
			percentile_cont(0.95) WITHIN GROUP (
				ORDER BY (telemetry->'metrics'->'cpu'->>'value')::float
			) as p95_cpu,
			COUNT(*) FILTER (
				WHERE last_seen > NOW() - INTERVAL '1 hour'
			) as recently_seen
		FROM assets a
		JOIN latest_telemetry lt ON lt.id = a.id
		WHERE telemetry->'metrics'->'cpu'->>'value' IS NOT NULL
		GROUP BY asset_type
	`)

	if err == nil {
		fmt.Printf("✓ Created materialized view in %v\n", time.Since(start))

		// Create index on materialized view
		pool.Exec(ctx, `
			CREATE INDEX IF NOT EXISTS idx_mv_asset_type
			ON mv_asset_telemetry_stats(asset_type)
		`)
	}

	// Query materialized view
	rows, err := pool.Query(ctx, `
		SELECT * FROM mv_asset_telemetry_stats
		ORDER BY total_assets DESC
		LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		fmt.Println("\n✓ Materialized view results:")
		for rows.Next() {
			var assetType string
			var total, recentlySeen int
			var avgCPU, maxCPU, p95CPU *float64

			rows.Scan(&assetType, &total, &avgCPU, &maxCPU, &p95CPU, &recentlySeen)

			fmt.Printf("  - %-10s: %d assets, ", assetType, total)
			if avgCPU != nil {
				fmt.Printf("CPU avg=%.1f%%, p95=%.1f%%", *avgCPU, *p95CPU)
			}
			fmt.Printf(", recent=%d\n", recentlySeen)
		}
	}

	// Refresh strategies
	fmt.Println("\n✓ Refresh strategies:")

	// Concurrent refresh (non-blocking)
	start = time.Now()
	_, err = pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_asset_telemetry_stats")
	if err != nil {
		// Need unique index for concurrent refresh
		pool.Exec(ctx, `
			CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_asset_type_unique
			ON mv_asset_telemetry_stats(asset_type)
		`)
		pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_asset_telemetry_stats")
	}
	fmt.Printf("  - Concurrent refresh: %v\n", time.Since(start))

	// Clean up
	pool.Exec(ctx, "DROP TABLE IF EXISTS site_summary")
	pool.Exec(ctx, "DROP MATERIALIZED VIEW IF EXISTS mv_asset_telemetry_stats")
}

func queryOptimization(ctx context.Context, pool *database.Pool) {
	fmt.Println("\n=== Query Optimization Techniques ===")

	// Enable timing for analysis
	pool.Exec(ctx, "SET track_io_timing = ON")

	// 1. Partial indexes for common queries
	fmt.Println("✓ Creating partial indexes...")

	_, err := pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_assets_active_lastseen
		ON assets(last_seen DESC)
		WHERE status = 'active'
	`)

	_, err = pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_assets_high_cpu
		ON assets((telemetry->'metrics'->'cpu'->>'value')::float)
		WHERE (telemetry->'metrics'->'cpu'->>'value')::float > 80
	`)

	// 2. Query planning analysis
	fmt.Println("\n✓ Query plan analysis:")

	var plan string
	err = pool.QueryRow(ctx, `
		EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)
		SELECT s.name, COUNT(a.id) as asset_count
		FROM sites s
		JOIN assets a ON a.site_id = s.id
		WHERE a.status = 'active'
			AND a.last_seen > NOW() - INTERVAL '1 hour'
		GROUP BY s.id, s.name
		ORDER BY asset_count DESC
		LIMIT 10
	`).Scan(&plan)

	if err == nil {
		// Parse and display key metrics from JSON plan
		fmt.Println("  - Query executed (JSON plan available)")
	}

	// 3. Common Table Expressions for complex queries
	fmt.Println("\n✓ CTE optimization example:")

	rows, err := pool.Query(ctx, `
		WITH RECURSIVE site_hierarchy AS (
			-- Simulate hierarchical data
			SELECT id, name, country, 0 as level
			FROM sites
			WHERE country = 'US'
			LIMIT 5

			UNION ALL

			SELECT s.id, s.name, s.country, sh.level + 1
			FROM sites s
			JOIN site_hierarchy sh ON s.country != sh.country
			WHERE sh.level < 2
				AND s.id NOT IN (SELECT id FROM site_hierarchy)
			LIMIT 10
		),
		asset_summary AS (
			SELECT
				site_id,
				COUNT(*) as count,
				jsonb_agg(DISTINCT asset_type) as types
			FROM assets
			GROUP BY site_id
		)
		SELECT
			sh.name,
			sh.country,
			sh.level,
			COALESCE(ast.count, 0) as asset_count,
			ast.types
		FROM site_hierarchy sh
		LEFT JOIN asset_summary ast ON ast.site_id = sh.id
		ORDER BY sh.level, sh.name
	`)

	if err == nil {
		defer rows.Close()
		fmt.Println("  - Hierarchical query with CTE executed")
	}

	// 4. Batch query optimization
	fmt.Println("\n✓ Optimization tips demonstrated:")
	fmt.Println("  1. Partial indexes reduce index size and improve performance")
	fmt.Println("  2. EXPLAIN ANALYZE shows actual execution statistics")
	fmt.Println("  3. CTEs can simplify complex queries and improve readability")
	fmt.Println("  4. Use covering indexes for index-only scans")
	fmt.Println("  5. Monitor pg_stat_statements for slow queries")

	// Show current connection stats
	var stats string
	pool.QueryRow(ctx, `
		SELECT json_build_object(
			'total_connections', count(*),
			'active_connections', count(*) FILTER (WHERE state = 'active'),
			'idle_connections', count(*) FILTER (WHERE state = 'idle'),
			'waiting_connections', count(*) FILTER (WHERE wait_event IS NOT NULL)
		)
		FROM pg_stat_activity
		WHERE datname = current_database()
	`).Scan(&stats)

	fmt.Printf("\n✓ Current connection stats: %s\n", stats)
}
