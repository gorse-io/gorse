# Data race in parallel feedback loading causes panic

## Error

`LoadDataFromDatabase` panics during model retraining, permanently stopping the retraining loop until the process is restarted:

```
{"level":"error","ts":1771404082.6079972,"msg":"panic recovered",
 "panic":"runtime error: index out of range [93] with length 93"}
```

The off-by-one (`[93]` with length `93`) is characteristic of a corrupted slice header: one goroutine reads a stale length while another is mid-`append` on the same slice.

## Root cause

`LoadDataFromDatabase` splits items into chunks (one per `NumJobs` goroutine) and fetches feedback per chunk in parallel. When a user has feedback for items in different chunks, multiple goroutines call `dataSet.AddFeedback()` for the same user concurrently. `AddFeedback` writes to shared slices without synchronization:

```go
// dataset/dataset.go:234-241
func (d *Dataset) AddFeedback(userId, itemId string, timestamp time.Time) {
    userIndex := d.userDict.Add(userId)          // ← concurrent map write
    itemIndex := d.itemDict.Add(itemId)          // ← concurrent map write
    d.userFeedback[userIndex] = append(...)       // ← concurrent slice append
    d.itemFeedback[itemIndex] = append(...)       // ← concurrent slice append
    d.timestamps[userIndex] = append(...)         // ← concurrent slice append
    d.numFeedback++                               // ← concurrent counter increment
}
```

Concurrent `append` on the same slice corrupts the slice header (length/capacity/pointer). This produces unpredictable behavior: silent data corruption, index out of range panics, or segfaults.

The existing `sync.Mutex` in the calling code only protects `posFeedbackCount` and `evaluator.Add` — it does not cover `AddFeedback`, `positiveSet[userIndex].Add()`, or the non-personalized recommender pushes.

## Reproducer

The following test reliably triggers the race (run with `-race`):

```go
func (s *MasterTestSuite) TestLoadDataFromDatabase_DataRace() {
    ctx := context.Background()
    s.Config = &config.Config{}
    s.Config.Recommend.CacheSize = 3
    s.Config.Recommend.DataSource.PositiveFeedbackTypes = []expression.FeedbackTypeExpression{
        expression.MustParseFeedbackTypeExpression("positive")}
    s.Config.Recommend.DataSource.ReadFeedbackTypes = []expression.FeedbackTypeExpression{
        expression.MustParseFeedbackTypeExpression("negative")}
    numJobs := 4
    if runtime.NumCPU() > numJobs {
        numJobs = runtime.NumCPU()
    }
    s.Config.Master.NumJobs = numJobs

    numItems := 100
    var items []data.Item
    for i := 0; i < numItems; i++ {
        items = append(items, data.Item{
            ItemId:     fmt.Sprintf("%07d", i),
            Timestamp:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
            Categories: []string{"*"},
        })
    }
    err := s.DataClient.BatchInsertItems(ctx, items)
    s.NoError(err)

    numUsers := 20
    var users []data.User
    for i := 0; i < numUsers; i++ {
        users = append(users, data.User{UserId: fmt.Sprintf("user_%d", i)})
    }
    err = s.DataClient.BatchInsertUsers(ctx, users)
    s.NoError(err)

    // Each user has feedback for items across ALL chunks
    var feedbacks []data.Feedback
    for u := 0; u < numUsers; u++ {
        for i := 0; i < numItems; i++ {
            feedbacks = append(feedbacks, data.Feedback{
                FeedbackKey: data.FeedbackKey{
                    UserId:       fmt.Sprintf("user_%d", u),
                    ItemId:       fmt.Sprintf("%07d", i),
                    FeedbackType: "positive",
                },
                Timestamp: time.Now(),
            })
        }
    }
    err = s.DataClient.BatchInsertFeedback(ctx, feedbacks, false, false, true)
    s.NoError(err)

    datasets, err := s.loadDataset(ctx)
    s.NoError(err)
    s.Equal(numUsers, datasets.rankingTrainSet.CountUsers())
    s.Equal(numItems, datasets.rankingTrainSet.CountItems())
    totalFeedback := datasets.rankingTrainSet.CountFeedback() + datasets.rankingTestSet.CountFeedback()
    s.Equal(numUsers*numItems, totalFeedback)
}
```

```
go test -race -run TestMaster/TestLoadDataFromDatabase_DataRace -v ./master/
```

## Race detector output (abbreviated)

```
WARNING: DATA RACE
Read at 0x00c000fcde88 by goroutine 75:
  dataset.(*Dataset).AddFeedback()
      dataset/dataset.go:240
  master.(*Master).LoadDataFromDatabase.func2()
      master/tasks.go:484

Previous write at 0x00c000fcde88 by goroutine 79:
  dataset.(*Dataset).AddFeedback()
      dataset/dataset.go:240
  master.(*Master).LoadDataFromDatabase.func2()
      master/tasks.go:484
```

Multiple races detected on lines 235, 237, 238, 239, 240 of `dataset/dataset.go` — all fields touched by `AddFeedback`.

## Impact

- **Panic**: corrupted slice headers cause index out of range at arbitrary points, most commonly in the `itemGroupIndex` linear scan or in `AddFeedback` itself
- **Silent data corruption**: even when no panic occurs, concurrent appends can lose feedback entries (a goroutine's append is overwritten by another), leading to incomplete training data
- **Permanent until restart**: the panic occurs inside `loadDataset` which stops the retraining loop; no automatic recovery

## Possible fixes

### Option A: Add mutex around all shared writes

Wrap `AddFeedback`, `positiveSet[userIndex].Add()`, and related calls in the existing mutex. This preserves parallelism for the DB fetch but serializes the processing — reducing the benefit of parallel loading.

### Option B: Single-pass fetch (recommended)

Replace the parallel per-chunk `GetFeedbackStream` calls with a single-pass fetch. Process all feedback in one goroutine using a pre-built `itemIdToPos` lookup map. This eliminates the race by construction — no shared writes, no mutex needed.

The total DB work is equivalent (one full scan vs N partial scans over the same index). Also replaces the O(n) linear scan for `itemGroupIndex` with O(1) map lookup.

This approach additionally fixes a secondary issue: the per-chunk range queries (`item_id >= first AND item_id <= last`) assume Go's bytewise sort order matches the database collation. When they differ (e.g. PostgreSQL `en_US.UTF-8`, MySQL `utf8mb4_0900_ai_ci`), chunk boundaries are misinterpreted, potentially dropping or duplicating feedback rows. The single-pass fetch makes no ordering assumptions.

---

*Disclosure: this analysis and fix were crafted with the assistance of [OpenAI Codex](https://openai.com/codex) and [Claude Code](https://claude.ai/code).*
