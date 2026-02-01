# PostgreSQL Playground 🐘

A hands-on PostgreSQL learning environment using Go, pgx, and sqlc. Built for engineers who want to master PostgreSQL's real-world features.

## Quick Start

```bash
# Start PostgreSQL and pgAdmin
make up

# Run migrations
make migrate

# Generate sqlc code
make generate

# Seed with 100k+ rows of realistic data
make seed

# Run examples
go run examples/01_basic_queries.go
go run examples/02_json_queries.go
go run examples/03_batch_operations.go
go run examples/04_advanced_patterns.go
```

Access pgAdmin at http://localhost:5050 (grug@cave.com / grug_password)

## What You'll Learn

### 1. **Basic Operations** (`examples/01_basic_queries.go`)
- Connection pooling with pgx
- Prepared statements
- JOIN queries (INNER, LEFT, complex)
- Window functions
- Aggregations with ROLLUP

### 2. **JSON/JSONB Mastery** (`examples/02_json_queries.go`)
- Query nested JSON with operators (`->`, `->>`, `#>`, `#>>`)
- JSON containment (`@>`, `<@`)
- Array operations with `jsonb_array_elements`
- GIN indexes for JSON
- Building JSON responses with `jsonb_build_object`

### 3. **Batch Operations** (`examples/03_batch_operations.go`)
- Multi-value INSERT (fast for <1000 rows)
- `COPY FROM` for bulk loading (100k+ rows/sec)
- Batch updates with `unnest()`
- Pipeline mode for maximum throughput
- Temporary tables for complex updates

### 4. **Advanced Patterns** (`examples/04_advanced_patterns.go`)
- Table partitioning by date range
- LISTEN/NOTIFY for real-time events
- Advisory locks for distributed coordination
- Materialized views with concurrent refresh
- Query optimization techniques

## Project Structure

```
postgres_playground/
├── docker-compose.yml      # PostgreSQL + pgAdmin
├── Makefile               # Common tasks
├── migrations/            # Schema versioning
│   ├── 001_initial_schema.up.sql
│   └── 001_initial_schema.down.sql
├── queries/               # sqlc SQL files
│   ├── sites.sql
│   └── assets.sql
├── pkg/database/          # Connection management
├── internal/db/           # Generated sqlc code
├── cmd/
│   ├── migrate/          # Migration runner
│   └── seed/             # Data generator
└── examples/             # Learning examples
```

## Schema Design

### Sites Table
- Physical locations with coordinates
- Messy JSONB metadata (simulating real-world data)
- GIN indexes for JSON queries

### Assets Table
- Hardware at sites (routers, switches, sensors)
- Complex JSONB for config and telemetry
- Foreign key to sites with CASCADE delete

## Real-World JSON Examples

Our seed data creates intentionally messy JSON to simulate production systems:

```json
// Old format from legacy system
{"type":"warehouse","manager":"John Smith","phone":"+1-555-123-4567","legacy_id":12345}

// New format with deep nesting
{"facility":{"type":"warehouse","classification":"A"},"contact":{"name":"Jane Doe","email":"jane@example.com"}}

// Mixed conventions
{"facilityType":"WAREHOUSE","Manager":"Bob","contact_phone":"+1-555-555-5555"}
```

## Performance Tips

1. **Indexes**: Use partial indexes for common WHERE clauses
2. **COPY**: Fastest bulk insert method (see benchmarks in examples)
3. **Prepared Statements**: Reuse for repeated queries
4. **Connection Pooling**: Configure based on workload
5. **EXPLAIN ANALYZE**: Your best friend for optimization

## Monitoring Queries

```sql
-- Slow queries
SELECT * FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 10;

-- Connection status
SELECT state, count(*)
FROM pg_stat_activity
GROUP BY state;

-- Index usage
SELECT schemaname, tablename, indexname, idx_scan
FROM pg_stat_user_indexes
ORDER BY idx_scan;
```

## Configuration Notes

The Docker Compose setup includes performance tuning:
- Shared memory and work_mem configured
- pg_stat_statements enabled
- Connection limits set appropriately

## Next Steps

1. Experiment with the examples
2. Check query plans with EXPLAIN ANALYZE
3. Try different index strategies
4. Build your own queries in `queries/`
5. Monitor performance with pg_stat_statements

## Philosophy

Following the grug manifesto:
- SQL is SQL (don't hide it)
- Understand what's happening
- Measure everything
- Keep it simple

Happy learning! 🚀