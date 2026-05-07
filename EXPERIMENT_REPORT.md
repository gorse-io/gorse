# Experiment: Reuse Redis Client for Valkey (Eliminate Code Duplication)

## Branch

```
experiment/valkey-reuse-redis-client
```

Based on: `feature/valkey-cache-integration`

Changes here do NOT affect the existing PR on `feature/valkey-cache-integration`.

---

## Context

A maintainer reviewed the Valkey integration PR and said:

> "Too many duplicate codes comparing to Redis backend. Please fix all tests. Could you reuse Redis client, but implement special timeseries interface if Valkey is detected?"

The original implementation (`storage/cache/valkey.go`, ~839 lines) used the `valkey-go` client library and reimplemented every method from scratch — Set, Get, Delete, Push, Pop, Scan, Purge, AddScores, SearchScores, DeleteScores, UpdateScores, ScanScores, AddTimeSeriesPoints, GetTimeSeriesPoints — all of which are semantically identical to the Redis implementation since Valkey is wire-compatible with Redis.

---

## What Was Done

### Approach: Embed `Redis`, override only time series

Since Valkey is wire-compatible with Redis for all commands except the TimeSeries module (`TS.ADD`, `TS.RANGE`), the solution is:

1. Connect to Valkey using the same `go-redis` client (it works out of the box)
2. Reuse ALL existing Redis methods (KV, queue, scores/search, scan, purge)
3. Override ONLY `AddTimeSeriesPoints` and `GetTimeSeriesPoints` with a sorted-set + hash implementation

### Files Changed

| File | Status | Description |
|------|--------|-------------|
| `storage/cache/redis_valkey.go` | **NEW** | Registers `valkey://`, `valkeys://`, `valkey+cluster://`, `valkeys+cluster://` prefixes. Defines `RedisValkey` type that embeds `Redis` and overrides only the two time series methods. ~160 lines total. |
| `storage/cache/redis.go` | **UNCHANGED** | Original Redis implementation is untouched. |
| `storage/cache/valkey.go` | **DISABLED** | Added `//go:build ignore` to exclude from compilation. File preserved for reference. |
| `storage/cache/valkey_test.go` | **DISABLED** | Added `//go:build ignore` to exclude from compilation. |
| `storage/cache/redis_test.go` | **MODIFIED** | Added `ValkeyViaRedisTestSuite` (tests valkey:// DSN end-to-end) and `RedisSortedSetTSTestSuite` (tests sorted-set TS implementation against a Redis server). |
| `storage/cache/database_test.go` | **MODIFIED** | Added comprehensive time series tests to `baseTestSuite`: duplicate timestamp, empty range, multiple series isolation, bucket aggregation correctness, single point, batch add, empty batch. Also added edge-case tests for AddScores(empty) and UpdateScores(no-op). |

### How `RedisValkey` Works

```go
type RedisValkey struct {
    Redis  // embeds all Redis methods
}
```

- `AddTimeSeriesPoints`: Uses pipelined `ZADD` (sorted set for timestamp index) + `HSET` (hash for values). Last-write-wins on duplicate timestamps via natural ZADD/HSET overwrite semantics.
- `GetTimeSeriesPoints`: Uses `ZRANGEBYSCORE` to get timestamps in range, `HMGET` to fetch values, then Go-side bucket aggregation (group by `floor(ts/duration)*duration`, keep last value per bucket).

This matches how the existing MongoDB backend implements time series in this codebase.

### Registration

In `redis_valkey.go` `init()`:
- `valkey://` / `valkeys://` → `redis.NewClient` (rewrites URL prefix)
- `valkey+cluster://` / `valkeys+cluster://` → `redis.NewClusterClient` (rewrites URL prefix)

All return a `*RedisValkey` which satisfies the `Database` interface.

---

## Test Suites

| Suite | What it tests | How to run |
|-------|---------------|------------|
| `TestRedis` | Original Redis with native TimeSeries | `go test ./storage/cache/ -run TestRedis -v` |
| `TestRedisSortedSetTS` | `RedisValkey` type against a Redis server (validates sorted-set TS logic using Redis's ZADD/HSET) | `go test ./storage/cache/ -run TestRedisSortedSetTS -v` |
| `TestValkeyViaRedis` | Full end-to-end via `valkey://` DSN against a real Valkey instance | `VALKEY_URI=valkey://127.0.0.1:6380/ go test ./storage/cache/ -run TestValkeyViaRedis -v` |

All suites run the shared `baseTestSuite` which covers every `Database` interface method.

---

## Next Steps

### 1. Compile and verify

```bash
go build ./storage/cache/...
```

If there are compilation issues (e.g., `go.mod` tidy needed because `valkey-go` is now unused by compiled code), run:

```bash
go mod tidy
```

### 2. Run tests against Redis

```bash
# Requires Redis with TimeSeries + Search modules on localhost:6379
go test ./storage/cache/ -run "TestRedis$" -v -count=1
go test ./storage/cache/ -run TestRedisSortedSetTS -v -count=1
```

Both should pass. `TestRedis` validates existing behavior is unbroken. `TestRedisSortedSetTS` validates the sorted-set fallback works correctly.

### 3. Run tests against Valkey

```bash
# Requires Valkey with valkey-search module on localhost:6380
VALKEY_URI=valkey://127.0.0.1:6380/ go test ./storage/cache/ -run TestValkeyViaRedis -v -count=1
```

Record any failures. Expected potential issues:
- `FT._LIST` / `FT.CREATE` / `FT.SEARCH` command compatibility (valkey-search module required)
- Minor behavioral differences in search result ordering or field formatting

### 4. If all tests pass

- Delete `storage/cache/valkey.go` and `storage/cache/valkey_test.go` entirely
- Remove `github.com/valkey-io/valkey-go` from `go.mod` + `go mod tidy`
- Run full project build: `go build ./...`
- Run full test suite to check nothing else breaks
- Update the PR on `feature/valkey-cache-integration` with these changes

### 5. If Valkey tests fail

- Document which tests fail and why
- Most likely cause: FT.SEARCH command differences between Redis Search and valkey-search
- Fix approach: adjust query formatting or result parsing in the inherited Redis methods (may need to override `SearchScores` in `RedisValkey` if behavior diverges)

---

## Key Design Decisions

1. **Embedding over interface**: Using Go struct embedding (`RedisValkey` embeds `Redis`) rather than a `timeSeriesBackend` interface. This avoids modifying `redis.go` at all — no new fields, no interface dispatch, no runtime detection.

2. **No changes to redis.go**: The maintainer's concern was code duplication. By embedding, we get zero duplication with zero changes to the working Redis implementation.

3. **go-redis over valkey-go**: Valkey speaks Redis protocol. Using go-redis means one client library, one set of patterns, one import. The `valkey-go` dependency can be removed entirely.

4. **Build tag on old files**: Using `//go:build ignore` rather than deleting `valkey.go`/`valkey_test.go` preserves them for reference during the experiment. Delete them once validated.

---

## File Locations (for reference)

```
storage/cache/
├── database.go          # Database interface definition
├── database_test.go     # Shared test suite (baseTestSuite) — MODIFIED
├── redis.go             # Original Redis implementation — UNCHANGED
├── redis_valkey.go      # NEW: RedisValkey type + valkey:// registration
├── redis_test.go        # Redis + Valkey test suites — MODIFIED
├── valkey.go            # OLD: standalone Valkey impl (//go:build ignore)
├── valkey_test.go       # OLD: standalone Valkey tests (//go:build ignore)
├── mongodb.go           # MongoDB implementation
├── sql.go               # SQL implementation
└── ...
```
