# Estimating `GET /api/recommend/{user}` response time

Context: follow-up to [performance-analysis.md](performance-analysis.md) for the deployment with ~50k users and ~200k
videos that reports more than 3 s per recommend request. This document collects reference latency numbers for each step
of the request, turns them into an estimation model, and records one new bottleneck found along the way. The request
path and the ranked bottlenecks (B1-B6) are explained in the performance analysis; this document only summarizes them.

## 1. The request in one page

Gorse is batch-first. The master rebuilds vectors and models every `fit_period` (60 min), and workers save a ranked list
per user (`recommend/<user>`) in the cache store about every minute. The request handler (`server/rest.go:1001`,
`logics/recommend.go`) then does:

| # | Step | Store calls | Always runs? |
|---|---|---|---|
| 1 | Load the user's full feedback history to build the "already seen" set (`recommend.go:63`) | 1 data store query, unbounded rows | Yes |
| 2 | Read the saved list: `SearchScores(recommend, user, categories, 0, cache_size=100)` (`recommend.go:108`) | 1 cache store query | Yes (unless `ranker.type = "none"`) |
| 3 | Drop seen items; return if `n + offset` items remain (`recommend.go:121`) | none | Yes |
| 4 | Fallback: for up to `context_size` (100) recent liked items, sequentially `GetVectors` + `QueryVectors(topK=101)` (`recommend.go:262-273`) | 2 vector calls per liked item, each a gRPC hop to the master when using `xvec://` | Only if step 3 came up short |
| 5 | Fallback: `latest` if still short | 1 data store query | Only if step 4 came up short |
| 6 | Write-back: one `BatchInsertFeedback` per returned item (`rest.go:1045-1056`) | n data store inserts | Only with `write-back-type` |

Who lands in step 4, and how expensive it is for them, is broken down in section 2.1. Reaching step 4 is not enough to
be slow: the cost scales with how many liked items the user has.

## 2. Why the slow steps are slow

### 2.1 Why the fallback (step 4) is slow

Several costs multiply together:

1. **Many small lookups instead of one.** The fallback takes up to `context_size` (100) of the user's most recent liked
   items. For each one it makes two vector calls: fetch that item's vector, then search for its 101 nearest neighbours.
   That is up to **200 calls for one request**; the fast path makes 2.
2. **One after another.** The loop (`logics/recommend.go:262-273`) waits for each search before starting the next. At
   ~9 ms per item (section 3.3), 100 items add up to ~0.9 s even with no other load.
3. **Every call is a network hop to the master.** With the default `xvec://` store, the vector index lives inside the
   master process. Servers cannot open it, so every fetch and search is a gRPC call to the master
   (`server/server.go:239-241`), adding a network round trip to each.
4. **The master answers one vector query at a time.** The xvec query concurrency is set from `master.n_jobs`, default
   **1** (`storage/vectors/xvec.go:93-99`). That one slot is shared by every server's fallback requests, every worker's
   offline pass (which runs the same per-item searches, uncapped), and the master's own rebuild tasks. A fallback request
   waits behind all of them. In section 3.3, 8 concurrent callers got no more throughput than 1; with 3-4 callers
   competing, ~0.9 s becomes ~3-4 s.
5. **Each search does more work than needed.**
   - `Xvec.QueryVectors` always filters on `hidden = false`, even without categories (`storage/vectors/xvec.go:408`).
     In section 3.3 that filter alone took a raw search from 4.1 ms to 7.4 ms.
   - It returns the full 1024-d vector for each of the 101 neighbours (`IncludeVectors: true`, `xvec.go:423`). The
     caller only uses IDs and scores and discards the vectors.

The number of liked items searched (`k`) depends on who falls back, so not every fallback is expensive:

| Who falls back | `k` | Cost |
|---|---|---|
| No positive feedback at all (cold start; the worker skips them, `worker/pipeline.go:193`) | 0: the loop runs zero times and falls through to `latest` | Cheap: one extra data store query |
| Has liked items but no CF embedding yet, because they first liked something after the last training run (`pipeline.go:206-208`) | Usually small, 1-10 | Moderate, until the next `fit_period` (60 min) plus a worker pass |
| Has a saved list, but it runs short after removing seen items or applying a **category filter** | Up to `context_size` (100): established users with long histories | **Most expensive**: the ~0.9 s to 3-4 s case |
| Inactive past `active_user_ttl`, list deleted (`pipeline.go:331-354`) | Up to 100 | Expensive, but only when `active_user_ttl` > 0; the default 0 disables deletion |
| Any user when `ranker.type = "none"` | n/a | The full recommender chain runs online on every request |

The category case is the likely trigger for a video site. The saved list holds only `cache_size` (100) items across all
categories, so `?category=...` often leaves fewer than `n` matches. A user who only browses never refreshes it either:
`GET /recommend` does not update `last_modify_user_time`, only writes such as inserting feedback do, so the worker has no
reason to rebuild their list.

```
fallback latency ≈ (liked items used, up to context_size)
                 × (search time, inflated by the always-on filter and returned vectors, plus network hop)
                 × (callers waiting for the single vector-query slot)
```

Shrinking any one factor helps immediately: lower `context_size` to 10-20 (fewer items), raise `master.n_jobs` (more
slots), or set the fallback to `["non-personalized/<x>", "latest"]` to skip vector searches entirely (~2-3 ms).

### 2.2 Other slow steps

- **Step 1 grows with the user's history.** No limit, no time window, `comment` and `labels` included. On a video site
  where every play is a feedback row, heavy users load thousands of rows on every request, fast path included.
- **`gorse-in-one` shares a process with training.** Full data reloads, model training and the per-user pass compete with
  request handling for CPU, memory (GC pauses) and the single vector-query slot.
- **Write-back is one insert per item**, so 10 returned items cost 10 extra round trips.

## 3. Reference metrics

### 3.1 Official Gorse REST benchmark

From the v0.4-era benchmark page, since removed from gorse.io and recovered from the
[`gorse-io/docs` git history](https://github.com/gorse-io/docs/blob/f72448b257febe0138a1e53710142241dd316e81~1/src/docs/benchmark.md).
Milliseconds per call; hardware not stated.

| Operation | MySQL data + Redis cache | Postgres data + Redis cache | MySQL data + MySQL cache | Postgres data + Postgres cache |
|---|---|---|---|---|
| `GetUserFeedback` (small history) | 0.32 | 0.31 | 0.38 | 0.34 |
| `GetFeedback/100` | 0.63 | 0.88 | 0.85 | 0.82 |
| `GetFeedback/1000` | 3.11 | 2.98 | 4.26 | 2.97 |
| `GetRecommendCache/100` | 0.50 | 0.48 | 3.05 | 1.30 |
| `RecommendFromOfflineCache/10` | 0.35 | 0.38 | 0.93 | 0.76 |
| `RecommendFromOfflineCache/100` | 0.59 | 0.60 | 1.98 | 1.49 |
| `RecommendFromLatest/10` | 0.69 | 0.71 | 1.47 | 1.23 |
| `RecommendFromItemBased/10` | 2.21 | 2.21 | 6.75 | 5.10 |
| `InsertFeedback/10` | 11.25 | 3.62 | 32.90 | 7.96 |

Caveat: in v0.4, item-to-item neighbours were precomputed into the cache store. `RecommendFromItemBased` is therefore a
cache read, not today's per-item vector-query fallback, and must not be used for step 4.

### 3.2 Production report: upstream issue #1175

[gorse-io/gorse#1175](https://github.com/gorse-io/gorse/issues/1175), Redis 8.6 + MySQL 8, mostly default config:

| Setup | `GET /recommend` |
|---|---|
| v0.4.16, 300 req/s, master training | **3 ms** |
| v0.5.4, master training | **15 s** |
| v0.5.4, master container stopped | 3 ms |
| v0.5.4, ranker disabled, only `user-to-user`, 1M users / 45M feedback rows | ~500 ms average, about double for heavy users |

The 15 s vs 3 ms split, driven only by whether the master is running, matches the shared vector-query slot and
`gorse-in-one`-style contention described in section 2.

### 3.3 Local probe of one fallback step

A short, one-off benchmark of step 4's per-item work (`GetVectors` + `QueryVectors(topK=101)`). Environment: Apple M4 Pro,
20k random 1024-d vectors in an HNSW index, gRPC over loopback. Random vectors are close to the worst case for HNSW, and a
200k-vector collection was not measured, so treat these as an order of magnitude only.

| Measurement | Time per step |
|---|---|
| Raw HNSW search, no filter | 4.1 ms |
| Raw HNSW search, `hidden = false` filter | 7.4 ms |
| Full step, direct call | 8.0-8.5 ms |
| Full step, through the master gRPC proxy | 8.9-9.4 ms |
| Full step with a category filter (1 in 20 items), through the proxy | 2.0 ms |
| 8 concurrent callers, `n_jobs = 1` | 8.2 ms per completed step (fully serialized) |
| 8 concurrent callers, `n_jobs = 8` | 1.3 ms per completed step (6.4x throughput) |

## 4. Estimation model

```
T ≈ T_feedback + T_cache                                     # steps 1-3, every request
  + [fallback] k × (T_step + 2 × RTT) × Q                    # step 4
  + [fallback, still short] T_latest                         # step 5
  + [write-back] n × T_insert                                # step 6

k  = min(context_size, number of the user's liked items)
Q  ≈ concurrent vector callers (other fallback requests + worker + master tasks) / master.n_jobs, at least 1
```

Values to plug in, from section 3 unless your own tracing says otherwise:

| Term | Value | Source |
|---|---|---|
| `T_feedback` | ~0.3 ms + ~3-4 ms per 1,000 feedback rows | 3.1 `GetUserFeedback`, `GetFeedback/1000` (extrapolated) |
| `T_cache` | ~0.5 ms Redis, ~1.3-3 ms SQL cache | 3.1 `GetRecommendCache/100` |
| `T_step` | ~8-9 ms at 20k vectors; likely more at 200k | 3.3 |
| `RTT` | your server-to-master network round trip | measure in your deployment |
| `T_latest` | ~0.7-1.5 ms without categories | 3.1 `RecommendFromLatest/10` |
| `T_insert` | ~1.4 ms or more per call on MySQL + Redis | 3.1 has no single-row feedback insert; `InsertUser` (1.39 ms) is the closest single-row write, and write-back makes one call per row |

### Worked scenarios (Redis cache, MySQL data)

| Scenario | Estimate |
|---|---|
| User has a saved list, 200 feedback rows | 0.3 + 0.6 + 0.5 ≈ **1-2 ms** |
| User has a saved list, 5,000 feedback rows | ~17 + 0.5 ≈ **~20 ms** |
| Fallback, 100 liked items, no contention (Q = 1) | 100 × ~9 ms ≈ **~0.9 s** |
| Fallback, 100 liked items, worker mid-pass plus 2 other fallback requests, `n_jobs = 1` (Q ≈ 3-4) | **~3-4 s** |
| Same, `n_jobs = 8` (Q = 1) and `context_size = 20` | 20 × ~9 ms ≈ **~0.2 s** |
| Fallback set to `["non-personalized/<x>", "latest"]` | 1 cache read + 1 data read ≈ **~2-3 ms** on top of the fast path |

The reported 3 s fits the fourth row: fallback requests queueing at the master's single vector-query slot. The fast path
alone would need close to a million feedback rows for one user to reach 3 s.

## 5. Replacing estimates with real numbers

1. **Per-endpoint latency:** the server exports the histogram `gorse_server_rest_api_request_seconds` with label
   `api="GET /api/recommend/{user-id}"` (only 200 responses are recorded, `server/rest.go:215-224`). Example p95:
   `histogram_quantile(0.95, sum by (le) (rate(gorse_server_rest_api_request_seconds_bucket{api="GET /api/recommend/{user-id}"}[5m])))`.
   It uses the Prometheus default buckets, so anything above 10 s lands in `+Inf`.
2. **Per-step split:** enable `[tracing] enable_tracing = true` with an OTLP exporter. REST, GORM, Redis and Mongo are
   already instrumented, so one slow trace shows `GetUserFeedback`, `SearchScores` and the fallback calls separately.
3. **Fast path vs fallback:** compare a user who has a `recommend/<user>` entry with one who does not. If only the second
   is slow, step 4 is the cause.
4. **Feedback volume:** count feedback rows for the slowest users to size `T_feedback`.

## 6. Levers, by effect on the model

| Change | Term it reduces | Kind |
|---|---|---|
| Raise `master.n_jobs` to the master's core count | `Q` | Config |
| Lower `recommend.context_size` to 10-20 | `k` | Config |
| Fallback to `["non-personalized/<x>", "latest"]` | removes step 4 | Config |
| External vector store (Qdrant/Milvus/Weaviate) instead of `xvec://` | `RTT` hop through master, `Q` | Config / infra |
| Workers with `--jobs` >= 8 | share of requests that reach step 4 | Config |
| Skip the `hidden = false` filter when no hidden items exist, or exclude hidden items at write time | `T_step` (~3 ms of ~9 ms in 3.3) | Code |
| Batch `GetVectors` and run `QueryVectors` concurrently; stop requesting `IncludeVectors` | `k × T_step` | Code |
| Cap and slim the feedback query (`positive_feedback_ttl`, row limit, fewer columns) | `T_feedback` | Code |
| One `BatchInsertFeedback` for write-back | `n × T_insert` | Code |
