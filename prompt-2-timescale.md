🦖 The Plan: TimescaleDB Evolution

We are going to turn your database into a time-series beast. Here is the "Grug" implementation plan:

### 1. Infrastructure (The Engine)
*   **What**: Swap standard Postgres for **TimescaleDB** (Postgres + Time superpowers).
*   **Why**: Free partitioning, 95% compression, auto-cleaning old data.
*   **How**: Update `docker-compose.yml` to use `timescale/timescaledb-ha:pg16`.

### 2. Schema (The Hypertable)
*   **Feature: Hypertables**. Looks like a table, acts like 1,000 small tables (chunks).
    *   *Implementation*: `create_hypertable('table_name', 'time_column')`.
*   **Feature: Compression**. Turn row data (fat) into column arrays (skinny).
    *   *Implementation*: `ALTER TABLE ... SET (timescaledb.compress)`.
*   **Feature: Continuous Aggregates**. Auto-updating materialized views.
    *   *Implementation*: `WITH (timescaledb.continuous)`.

### 3. Usage (The Code)
*   **Ingest**: We will write a specific **Bulk Ingest** example using `CopyFrom` because `INSERT` is too slow for time-series.
*   **Query**: We will use `time_bucket()` (histogram for time) and `gapfill()` (fixing holes in data).
*   **Low Latency**: We will use **SkipScan** to instantly find the "last known value" for 10,000 sensors.

Let's execute!

---

### Step 1: Update Docker Image

We switch to the official TimescaleDB image. It contains the extension pre-loaded.
