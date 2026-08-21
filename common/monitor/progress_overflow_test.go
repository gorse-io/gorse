package monitor

import (
	"context"
	"testing"
)

// TestProgress_ZeroTotalChildDoesNotExceed100Percent pins the behavior of the
// zero-total child fallback in Span.Progress. Before the clamp, a parent
// with total=2,count=2 plus a running child with total=0 would return
// Count=3,Total=2 (150%), which confused the dashboard progress bar.
//
// The clamp ensures Count <= Total for every case the fallback handles.
func TestProgress_ZeroTotalChildDoesNotExceed100Percent(t *testing.T) {
	_, parent := Start(context.Background(), "parent", 2)
	parent.Add(2) // parent's own work is complete

	ctx := context.WithValue(context.Background(), spanKeyName, parent)
	_, _ = Start(ctx, "child", 0)

	got := parent.Progress()
	if got.Count > got.Total {
		t.Fatalf("Progress.Count (%d) exceeds Progress.Total (%d)", got.Count, got.Total)
	}
}
