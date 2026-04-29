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
| 9 | Performance benchmarks — time series | Create benchmarks comparing Valkey time series (sorted set + Go aggregation) vs Redis TimeSeries for ingestion throughput, range query latency, and aggregation performance. Document results and gaps. | 3h | ✅ Done |
| 10 | Documentation and PR preparation | Update README or contributing docs if needed. Final code review, cleanup, and PR submission to gorse-io/gorse. | 2h | To Do |

**Total estimate: ~33 hours (~4 working days)**

- Tasks 2–3 (foundation): 3h — half day
- Tasks 4–7 (core implementation): 17h — ~2 days
- Task 8 (tests): 4h — half day
- Tasks 9–10 (benchmarks + PR): 5h — ~1 day

---

## 7. Performance Benchmark Results — Time Series: Valkey vs Redis

### 7.1 Test Environment

- **CPU**: Intel Core i7-9750H @ 2.60GHz (12 threads)
- **OS**: macOS (darwin/amd64)
- **Go**: 1.26.1
- **Redis**: redis/redis-stack:latest (port 6379) — uses Redis TimeSeries module
- **Valkey**: valkey/valkey-bundle:unstable (port 6380) — uses sorted set + hash with Go-side aggregation
- **Benchmark tool**: Go `testing.B` with `-benchtime 10s`
- **Runs**: 3 independent runs per backend, plus 1 side-by-side run

### 7.2 What Was Compared

| | Redis | Valkey |
|---|---|---|
| **Ingestion** | `TS.ADD` (single native command, pipelined) | `ZADD` + `HSET` (two commands, pipelined) |
| **Range query** | `TS.RANGE` with server-side `LAST` aggregation | `ZRANGEBYSCORE` + `HMGET` + Go-side bucketing |
| **Duplicate handling** | `DUPLICATE_POLICY LAST` (server-side) | `ZADD` overwrites score, `HSET` overwrites field (natural last-write-wins) |

### 7.3 Results (side-by-side run)

| Benchmark | Description | Redis (ns/op) | Valkey (ns/op) | Ratio |
|---|---|---|---|---|
| **TSIngestSingle** | 1 point per call | 361,638 | 367,864 | **1.02x** |
| **TSIngestBatch** | 100 points per call | 3,827,686 | 1,236,382 | **0.32x** ✅ |
| **TSQuerySmallRange** | 60s window, 1s buckets, 10K pts loaded | 399,321 | 927,515 | **2.3x** |
| **TSQueryLargeRange** | Full range, 60s buckets, 10K pts loaded | 531,244 | 29,022,625 | **54.6x** |
| **TSQueryWideRange** | Full range, 1h buckets, 100K pts loaded | 1,346,747 | 332,029,964 | **246.6x** |

### 7.4 Results (averaged across 3 independent runs)

| Benchmark | Redis avg (ns/op) | Valkey avg (ns/op) | Ratio |
|---|---|---|---|
| **TSIngestSingle** | 361,138 | 381,977 | **1.06x** |
| **TSIngestBatch** | 3,796,799 | 1,214,417 | **0.32x** ✅ |
| **TSQuerySmallRange** | 394,036 | 910,520 | **2.3x** |
| **TSQueryLargeRange** | 512,476 | 28,634,833 | **55.9x** |
| **TSQueryWideRange** | 1,351,768 | 338,763,674 | **250.7x** |

### 7.5 Analysis

**Ingestion (single point)** — Effectively identical (1.02–1.06x). The overhead of two commands (ZADD + HSET) vs one (TS.ADD) is negligible at the single-operation level due to network round-trip dominating.

**Batch ingestion** — Valkey is 3.1x faster. The Valkey pipeline batches ZADD + HSET pairs efficiently, while Redis TimeSeries TS.ADD has higher per-command overhead in pipeline mode. This is a genuine advantage for bulk loading scenarios.

**Small range queries (60s window)** — 2.3x slower. Valkey fetches ~60 raw points via ZRANGEBYSCORE + HMGET and aggregates in Go. Redis returns pre-aggregated results server-side. The absolute latency (~0.9ms) is well within acceptable bounds for monitoring queries.

**Large range queries (10K points, 60s buckets)** — 55x slower. This is where the architectural difference shows: Valkey must transfer all 10,000 raw points to the client, then bucket-aggregate in Go. Redis aggregates server-side and returns only ~167 bucket results. Absolute latency: ~29ms vs ~0.5ms.

**Wide range queries (100K points, 1h buckets)** — 247x slower. The worst case: 100,000 points transferred over the network before Go-side aggregation into ~28 buckets. Absolute latency: ~332ms vs ~1.3ms.

### 7.6 Root Cause of Query Gaps

The performance gap in range queries scales linearly with the number of raw points in the range. The bottleneck is **data transfer**, not computation:

1. Redis TimeSeries aggregates server-side → returns only bucket results (28–167 values)
2. Valkey transfers all raw points → client does aggregation (10K–100K values over the network)

The Go-side bucketing itself is fast (simple map + sort). The cost is dominated by `ZRANGEBYSCORE` returning all members + `HMGET` fetching all values.

### 7.7 Impact on Gorse

Gorse uses time series for **monitoring metrics** (NDCG, precision, recall, feedback ratios, task durations). These are characterized by:

- **Low volume**: Metrics are recorded per training cycle or per evaluation, not per-second. A typical deployment might have hundreds to low thousands of points per metric, not 100K.
- **Infrequent queries**: Dashboard queries happen on user interaction, not in hot paths.
- **Moderate ranges**: Most dashboard views show hours to days of data, not months.

For Gorse's workload profile:
- Single-point ingestion and batch ingestion are **on par or better** than Redis.
- Small range queries (~0.9ms) are **fast enough** for dashboard rendering.
- Large range queries (~29ms) are **acceptable** for occasional dashboard use.
- Wide range queries (~332ms) would only occur with unusually large datasets and are still **sub-second**.

### 7.8 Optimization Path

If future workloads require faster large-range aggregation, **Approach B (Lua script server-side aggregation)** can be implemented as a drop-in replacement for `GetTimeSeriesPoints`. The Lua script would execute ZRANGEBYSCORE + HGET + bucketing entirely on the Valkey server, eliminating the data transfer bottleneck. This was documented in Section 2.3 of this report.

### 7.9 How to Reproduce

```bash
# Prerequisites: Redis Stack on port 6379, Valkey on port 6380
docker run -d --name redis-stack -p 6379:6379 redis/redis-stack:latest
docker run -d --name valkey-search -p 6380:6379 valkey/valkey-bundle:unstable

# Run side-by-side
VALKEY_URI="valkey://127.0.0.1:6380/" go test -v ./storage/cache/ \
  -run ^$ -bench "BenchmarkRedis/TS|BenchmarkValkey/TS" -benchtime 10s -timeout 10m

# Run individually
go test -v ./storage/cache/ -run ^$ -bench "BenchmarkRedis/TS" -benchtime 10s -timeout 10m
VALKEY_URI="valkey://127.0.0.1:6380/" go test -v ./storage/cache/ \
  -run ^$ -bench "BenchmarkValkey/TS" -benchtime 10s -timeout 10m
```
