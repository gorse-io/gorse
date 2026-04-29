# Valkey Integration Report for Gorse Recommender System

## 1. Why No Valkey for the Data Layer (`storage/data/Database`)

The `data.Database` interface defines 25+ methods for users, items, and feedback with complex relational semantics: filtered pagination, category-based batch lookups, time-range queries, cross-entity joins, and streaming exports. Every existing backend (MySQL, PostgreSQL, ClickHouse, SQLite, MongoDB) handles these natively via SQL or document queries. No Redis-based data backend exists in this project. Implementing these patterns on Valkey would mean building an application-level query engine on top of key-value primitives — the engineering cost would be high and the result would be slower and more fragile than any existing backend. Valkey is a natural fit for the cache layer, not the data layer.

---

## 2. Time Series Implementation in Valkey

### 2.1 Context

The `cache.Database` interface requires two time series methods:

- **`AddTimeSeriesPoints(ctx, points []TimeSeriesPoint)`** — Ingests points with (Name, Timestamp, Value). Duplicate policy: last write wins.
- **`GetTimeSeriesPoints(ctx, name, begin, end, duration)`** — Retrieves points in a time range, aggregated into buckets of `duration` width, returning the last value per bucket.

The existing Redis implementation uses Redis TimeSeries module commands (`TS.ADD` with `LAST` duplicate policy, `TS.RANGE` with `LAST` aggregator and `BucketDuration`). Valkey has no TimeSeries module, so we need an alternative.

### 2.2 Approach A: Sorted Sets with Go-Side Aggregation (Recommended)

**Design:**
- One sorted set per time series name: key = `{prefix}time_series_points:{name}`
- Score = timestamp in milliseconds (Unix epoch)
- Member = string-encoded float64 value (using `strconv.FormatFloat`)
- For duplicate timestamps (last-write-wins): use a composite member format `{timestamp_ms}` as the score and the value as the member. Since ZADD with the same member updates the score, we instead encode as member = `{timestamp_ms}` and store value in a parallel hash, OR use a simpler approach: member = `fmt.Sprintf("%d", timestamp_ms)` and value stored via a separate HSET. However, the simplest approach that matches the "LAST" duplicate policy is:
  - Member = `fmt.Sprintf("%d", timestamp.UnixMilli())` (timestamp as string)
  - Score = `timestamp.UnixMilli()` (float64)
  - Value stored in a parallel hash: `HSET {prefix}time_series_values:{name} {timestamp_ms} {value}`

**Actually, the cleanest approach** (matching how MongoDB does it in this codebase):
- Use a single sorted set per series name
- Member = `fmt.Sprintf("%d:%.17g", timestamp.UnixMilli(), value)` — encode both timestamp and value in the member
- Score = `float64(timestamp.UnixMilli())`
- For "last write wins" on duplicate timestamps: first `ZREMRANGEBYSCORE` the exact timestamp, then `ZADD` the new point. This can be done atomically in a pipeline.

**Simpler alternative** (recommended for clarity):
- Use Hash + Sorted Set combination:
  - Sorted Set key: `{prefix}ts_index:{name}` — score = timestamp_ms, member = timestamp_ms as string
  - Hash key: `{prefix}ts_data:{name}` — field = timestamp_ms as string, value = float64 as string
- `AddTimeSeriesPoints`: Pipeline of ZADD (index) + HSET (data) per point. ZADD naturally handles duplicate timestamps (updates score), HSET naturally overwrites (last-write-wins).
- `GetTimeSeriesPoints`: ZRANGEBYSCORE to get timestamp members in range, HMGET to fetch values, then bucket aggregation in Go.

**Go-side aggregation logic** (for `GetTimeSeriesPoints`):
```go
// 1. Fetch all timestamps in range from sorted set
// 2. Fetch corresponding values from hash
// 3. Group by bucket: bucket_key = (timestamp_ms / duration_ms) * duration_ms
// 4. For each bucket, take the last value (highest timestamp)
// 5. Return sorted results
```

This matches exactly how MongoDB implements it — MongoDB uses `$group` with `$floor($divide)` for bucketing and `$top` with `sortBy: {timestamp: -1}` for last-value selection.

**Benefits:**
- Simple, well-understood data structures (sorted set + hash)
- Exact semantic match with Redis TimeSeries "LAST" duplicate policy
- Proven pattern — MongoDB implementation in this codebase does the same thing
- No external modules required — works with any Valkey version
- Efficient range queries via ZRANGEBYSCORE: O(log N + M)
- Natural duplicate handling via ZADD + HSET overwrite semantics

**Downsides:**
- Aggregation happens in Go, not server-side — transfers more data over the network for large ranges
- Two keys per series (index + data) instead of one — slightly more memory overhead
- No built-in downsampling or retention policies (must be managed application-side)
- For very large time ranges with fine-grained data, the full range must be fetched before aggregation

---

### 2.3 Approach B: Sorted Sets with Lua Script Aggregation (Server-Side)

**Design:**
- Same storage layout as Approach A (sorted set + hash)
- Aggregation performed via a Lua script executed with `EVALSHA`/`InvokeScript`
- The Lua script does: ZRANGEBYSCORE, HGET for each member, bucket grouping, and returns aggregated results

**Lua script pseudocode:**
```lua
local key_index = KEYS[1]  -- sorted set key
local key_data = KEYS[2]   -- hash key
local begin_ms = tonumber(ARGV[1])
local end_ms = tonumber(ARGV[2])
local bucket_ms = tonumber(ARGV[3])

local members = redis.call('ZRANGEBYSCORE', key_index, begin_ms, end_ms)
local buckets = {}
local bucket_order = {}

for _, member in ipairs(members) do
    local ts = tonumber(member)
    local value = redis.call('HGET', key_data, member)
    local bucket_key = math.floor(ts / bucket_ms) * bucket_ms
    
    if not buckets[bucket_key] or ts > buckets[bucket_key].ts then
        if not buckets[bucket_key] then
            table.insert(bucket_order, bucket_key)
        end
        buckets[bucket_key] = {ts = ts, value = value}
    end
end

table.sort(bucket_order)
local result = {}
for _, bk in ipairs(bucket_order) do
    table.insert(result, tostring(bk))
    table.insert(result, buckets[bk].value)
end
return result
```

**Benefits:**
- Aggregation happens server-side — reduces network transfer for large datasets
- Atomic operation — no race conditions during read
- Single round-trip for the entire query
- valkey-glide supports `InvokeScript` for Lua execution

**Downsides:**
- Lua scripts add complexity to maintain and debug
- Lua execution blocks the Valkey event loop — long-running aggregations on large datasets could cause latency spikes
- Script must be loaded/cached on each Valkey node (handled by glide's InvokeScript)
- Harder to unit test the aggregation logic independently
- Lua in Valkey has limited floating-point precision (uses double, but string conversion can lose precision)

---

### 2.4 Approach C: Single Sorted Set with Encoded Members

**Design:**
- One sorted set per series: key = `{prefix}ts:{name}`
- Score = `float64(timestamp.UnixMilli())`
- Member = `fmt.Sprintf("%.17g", value)` — just the value
- For duplicate timestamps: ZADD with GT flag won't work (we want last-write-wins, not max). Instead, remove old entry at same score first, then add new one.

**Problem:** Sorted sets don't support multiple members with the same score cleanly for "replace" semantics. If two different values have the same timestamp, ZADD treats them as different members. You'd need ZREMRANGEBYSCORE + ZADD in a pipeline, which is what Approach A avoids by using a hash for values.

**Benefits:**
- Single key per series (simpler key space)
- Slightly less memory than two-key approach

**Downsides:**
- Duplicate timestamp handling is awkward and error-prone
- Can't efficiently update a value at an existing timestamp without removing all entries at that score
- Member = value means you can't have the same value at different timestamps (sorted set members are unique)
- **This approach has fundamental correctness issues** — not recommended

---

### 2.5 Approach D: Hash-Only with Go-Side Sorting and Aggregation

**Design:**
- Single hash per series: key = `{prefix}ts:{name}`
- Field = timestamp_ms as string, Value = float64 as string
- `AddTimeSeriesPoints`: HSET with field=timestamp, value=value (natural last-write-wins)
- `GetTimeSeriesPoints`: HGETALL, filter by range in Go, sort, bucket, aggregate

**Benefits:**
- Simplest storage model — single key, single data structure
- Natural last-write-wins via HSET overwrite
- No sorted set overhead

**Downsides:**
- HGETALL fetches ALL points for the series, not just the requested range — very inefficient for large series
- No server-side range filtering — all data must be transferred to client
- Sorting must be done in Go after fetching all data
- Memory usage grows unbounded without manual cleanup
- **Not viable for production workloads with large time series**

---

## 3. Decision

**Approach A (Sorted Sets + Hash with Go-Side Aggregation)** — confirmed. If benchmarks later show performance is not on par with Redis TimeSeries, Approach B (Lua script server-side aggregation) can be explored as an optimization.

---

## 4. Client Library: valkey-glide Go v2

The implementation will use `github.com/valkey-io/valkey-glide/go/v2` — the official Valkey GLIDE client for Go.

**Key API mappings for our implementation:**

| Gorse Operation | valkey-glide API |
|---|---|
| Set key-value | `client.Set(ctx, key, value)` |
| Get key-value | `client.Get(ctx, key)` → `models.Result[string]` |
| Delete key | `client.Del(ctx, []string{key})` |
| Hash set | `client.HSet(ctx, key, map[string]string{...})` |
| Hash get all | `client.HGetAll(ctx, key)` → `map[string]string` |
| Hash get field | `client.HGet(ctx, key, field)` → `models.Result[string]` |
| Hash delete | `client.HDel(ctx, key, []string{...})` |
| Sorted set add | `client.ZAdd(ctx, key, map[string]float64{...})` |
| Sorted set range with scores | `client.ZRangeWithScores(ctx, key, query)` → `[]models.MemberAndScore` |
| Sorted set remove by score | `client.ZRemRangeByScore(ctx, key, rangeQuery)` |
| Sorted set pop min | `client.ZPopMin(ctx, key)` → `map[string]float64` |
| Sorted set cardinality | `client.ZCard(ctx, key)` |
| Scan keys | `client.Scan(ctx, cursor)` / `client.ScanWithOptions(ctx, cursor, opts)` |
| Pipeline/batch | `client.Exec(ctx, batch, raiseOnError)` with `pipeline.NewStandaloneBatch(false)` |
| Lua scripting | `client.InvokeScript(ctx, script)` / `client.InvokeScriptWithOptions(ctx, script, opts)` |
| Watch (optimistic locking) | `client.Watch(ctx, keys)` |
| Custom command (FT.*, etc.) | `client.CustomCommand(ctx, []string{...})` |
| Ping | `client.Ping(ctx)` |
| Close | `client.Close()` |

**Connection setup:**
```go
import (
    glide "github.com/valkey-io/valkey-glide/go/v2"
    "github.com/valkey-io/valkey-glide/go/v2/config"
)

// Standalone
client, err := glide.NewClient(&config.ClientConfiguration{
    Addresses: []config.NodeAddress{{Host: host, Port: port}},
})

// Cluster
client, err := glide.NewClusterClient(&config.ClusterClientConfiguration{
    Addresses: []config.NodeAddress{{Host: host, Port: port}},
})
```

**Key differences from go-redis (current Redis client):**
- No `ParseURL` — connection params must be parsed manually from the URL
- `ZAdd` takes `map[string]float64` instead of `[]redis.Z`
- `Get` returns `models.Result[string]` with `.Value()` and `.IsNil()` methods
- Pipeline uses `pipeline.NewStandaloneBatch(false)` + `client.Exec()`
- No direct FT.* (RediSearch) methods — must use `CustomCommand` for valkey-search
- Context is required for all operations (consistent with Go best practices)

---

## 5. Implementation Plan

### Phase 1: Foundation
1. Add Valkey URL prefix constants to `storage/scheme.go`
2. Update config validation in `config/config.go`
3. Update `config/config.toml` documentation

### Phase 2: Core Implementation
1. Create `storage/cache/valkey.go` — full `cache.Database` implementation
2. Implement connection setup (standalone + cluster) using valkey-glide
3. Implement core operations: Set/Get/Delete, Push/Pop/Remain, Scan/Purge
4. Implement score operations using `CustomCommand` for valkey-search FT.* commands
5. Implement time series using Sorted Set + Hash with Go-side aggregation

### Phase 3: Tests
1. Create `storage/cache/valkey_test.go`
2. Run existing `baseTestSuite` against Valkey

### Phase 4: Benchmarks
1. Time series ingestion throughput comparison
2. Range query + aggregation latency comparison
3. Document results

### Files to Create/Modify

| File | Action |
|---|---|
| `storage/scheme.go` | Add Valkey prefix constants |
| `storage/cache/valkey.go` | New — full implementation |
| `storage/cache/valkey_test.go` | New — test suite |
| `config/config.go` | Add Valkey to cache_store validator |
| `config/config.toml` | Document Valkey support |
| `go.mod` | Add valkey-glide dependency |


---

## 6. Task Breakdown

### Story: Add Valkey Support to Gorse Recommender System (Cache Layer)

| # | Task | Description | Estimate | Status |
|---|---|---|---|---|
| 1 | Initial Valkey integration exploration | Explore Gorse project architecture, analyze cache and data layer interfaces, evaluate Valkey compatibility, research valkey-glide Go client API, produce integration report with time series approach options and implementation plan. | 4h | ✅ Done |
| 2 | Add Valkey URL prefix constants and config validation | Add `valkey://`, `valkeys://`, `valkey+cluster://`, `valkeys+cluster://` prefixes to `storage/scheme.go`. Update `config/config.go` cache_store validator to accept Valkey prefixes. Update `config/config.toml` to document Valkey as a supported cache store option. Add config validation tests. | 2h | ✅ Done |
| 3 | Add valkey-glide Go dependency | Add `github.com/valkey-io/valkey-glide/go/v2` to `go.mod`. Run `go mod tidy`. Verify build compiles. | 1h | ✅ Done |
| 4 | Implement Valkey cache backend — connection and lifecycle | Create `storage/cache/valkey.go`. Implement `init()` registration with Valkey prefixes, connection setup for standalone and cluster modes using valkey-glide, `Close()`, `Ping()`, `Init()` (valkey-search index creation via CustomCommand), `Scan()`, `Purge()`. | 4h | ✅ Done |
| 5 | Implement Valkey cache backend — core key-value and queue operations | Implement `Set()`, `Get()`, `Delete()` for key-value metadata. Implement `Push()`, `Pop()`, `Remain()` for message queue using sorted sets. | 3h | ✅ Done |
| 6 | Implement Valkey cache backend — score/document operations | Implement `AddScores()`, `SearchScores()`, `UpdateScores()`, `DeleteScores()`, `ScanScores()` using valkey-search FT.* commands via CustomCommand. | 6h | ✅ Done |
| 7 | Implement Valkey cache backend — time series operations | Implement `AddTimeSeriesPoints()` using sorted set + hash with pipeline batching. Implement `GetTimeSeriesPoints()` with ZRANGEBYSCORE range fetch and Go-side bucket aggregation (Approach A). | 4h | ✅ Done |
| 8 | Create Valkey cache test suite | Create `storage/cache/valkey_test.go`. Embed `baseTestSuite` and run all existing cache interface tests against a Valkey instance. Use `VALKEY_URI` environment variable. | 4h | ✅ Done |
| 9 | Performance benchmarks — time series | Create benchmarks comparing Valkey time series (sorted set + Go aggregation) vs Redis TimeSeries for ingestion throughput, range query latency, and aggregation performance. Document results and gaps. | 3h | To Do |
| 10 | Documentation and PR preparation | Update README or contributing docs if needed. Final code review, cleanup, and PR submission to gorse-io/gorse. | 2h | To Do |

**Total estimate: ~33 hours (~4 working days)**

- Tasks 2–3 (foundation): 3h — half day
- Tasks 4–7 (core implementation): 17h — ~2 days
- Task 8 (tests): 4h — half day
- Tasks 9–10 (benchmarks + PR): 5h — ~1 day

---

## 7. Performance Benchmark Plan — Time Series: Valkey vs Redis

### 7.1 Goal

Compare the performance of Valkey's time series implementation (sorted set + hash with Go-side aggregation) against Redis TimeSeries (native TS.ADD / TS.RANGE with server-side aggregation). The benchmarks should answer: is the Valkey approach fast enough for Gorse's workload, and where are the gaps?

### 7.2 What We're Comparing

| | Redis | Valkey |
|---|---|---|
| **Ingestion** | `TS.ADD` (single native command) | `ZADD` + `HSET` (two commands, pipelined) |
| **Range query** | `TS.RANGE` with server-side `LAST` aggregation | `ZRANGEBYSCORE` + `HMGET` + Go-side bucketing |
| **Duplicate handling** | `DUPLICATE_POLICY LAST` (server-side) | `ZADD` overwrites score, `HSET` overwrites field (natural last-write-wins) |

### 7.3 Benchmark Scenarios

All benchmarks use Go's `testing.B` framework and run against both Redis (port 6379) and Valkey (port 6380) from the same test file. Each scenario is a separate `b.Run` sub-benchmark.

**Scenario 1: Ingestion throughput — single point**
- Insert one point per iteration (`b.N` iterations)
- Measures: ops/sec for single-point writes
- Simulates real-time metric ingestion (one metric update at a time)

**Scenario 2: Ingestion throughput — batch**
- Insert 100 points per iteration in a single call
- Measures: ops/sec for batch writes, points/sec throughput
- Simulates bulk metric loading (e.g., after model training completes)

**Scenario 3: Range query — small range, no aggregation**
- Pre-load 10,000 points (1 point per second over ~2.7 hours)
- Query a 60-second window with 1-second bucket duration (returns ~60 points, no aggregation needed)
- Measures: query latency when bucket size equals point interval

**Scenario 4: Range query — large range with aggregation**
- Pre-load 10,000 points (1 point per second)
- Query the full range with 60-second bucket duration (aggregates ~10,000 points into ~167 buckets)
- Measures: query latency with heavy aggregation — this is where Go-side vs server-side aggregation matters most

**Scenario 5: Range query — wide range, coarse aggregation**
- Pre-load 100,000 points (1 point per second over ~27 hours)
- Query the full range with 3600-second (1 hour) bucket duration (aggregates into ~28 buckets)
- Measures: worst-case scenario — large data transfer + aggregation

### 7.4 Implementation

Add time series benchmarks to `storage/cache/database_test.go` as shared functions (like the existing `benchmark()` pattern), then call them from both `redis_test.go` and `valkey_test.go`.

**New shared functions in `database_test.go`:**
```
benchmarkTimeSeriesIngestSingle(b, database)
benchmarkTimeSeriesIngestBatch(b, database)
benchmarkTimeSeriesQuerySmallRange(b, database)
benchmarkTimeSeriesQueryLargeRange(b, database)
benchmarkTimeSeriesQueryWideRange(b, database)
```

**Wired into existing benchmark functions in `redis_test.go` and `valkey_test.go`:**
```go
// In BenchmarkRedis / BenchmarkValkey:
b.Run("TSIngestSingle", func(b *testing.B) { benchmarkTimeSeriesIngestSingle(b, database) })
b.Run("TSIngestBatch", func(b *testing.B) { benchmarkTimeSeriesIngestBatch(b, database) })
b.Run("TSQuerySmallRange", func(b *testing.B) { benchmarkTimeSeriesQuerySmallRange(b, database) })
b.Run("TSQueryLargeRange", func(b *testing.B) { benchmarkTimeSeriesQueryLargeRange(b, database) })
b.Run("TSQueryWideRange", func(b *testing.B) { benchmarkTimeSeriesQueryWideRange(b, database) })
```

### 7.5 How to Run

```bash
# Run both side by side
go test -v ./storage/cache/ -run ^$ -bench "BenchmarkRedis/TS|BenchmarkValkey/TS" -benchtime 10s -timeout 10m

# Redis only
go test -v ./storage/cache/ -run ^$ -bench "BenchmarkRedis/TS" -benchtime 10s -timeout 10m

# Valkey only
go test -v ./storage/cache/ -run ^$ -bench "BenchmarkValkey/TS" -benchtime 10s -timeout 10m
```

### 7.6 Expected Outcomes

- **Ingestion**: Valkey will likely be slightly slower due to 2 commands (ZADD + HSET) vs 1 (TS.ADD), but pipelining should keep the gap small.
- **Small range queries**: Should be comparable — both fetch a small number of points, minimal aggregation.
- **Large range queries with aggregation**: Redis TimeSeries will likely be faster since aggregation happens server-side. Valkey must transfer all raw points to the client before aggregating. This is the key gap to quantify.
- **Gorse context**: Gorse uses time series for monitoring metrics (NDCG, precision, recall, feedback ratios) — typically low-volume data with infrequent queries. Even a 2-5x gap on large aggregation queries is acceptable for this workload.

### 7.7 Documenting Results

After running, capture output in a results table:

```
| Benchmark | Redis (ns/op) | Valkey (ns/op) | Ratio |
|---|---|---|---|
| TSIngestSingle | | | |
| TSIngestBatch | | | |
| TSQuerySmallRange | | | |
| TSQueryLargeRange | | | |
| TSQueryWideRange | | | |
```

If any scenario shows >10x degradation, document the root cause and note Approach B (Lua script server-side aggregation) as the optimization path.

### 7.8 After Running Benchmarks

1. Paste the benchmark output into the results table above
2. Commit the benchmark code and results to the feature branch
3. Proceed to Task 10 — PR preparation:
   - Squash or clean up commits if needed
   - Write PR description referencing the ticket
   - Submit PR to `gorse-io/gorse`
