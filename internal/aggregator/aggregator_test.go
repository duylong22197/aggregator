package aggregator

import (
	"context"
	"testing"
	"time"

	"github.com/duylong22197/aggregator/internal/models"
)

// feedJobs is a test helper that sends rows into a channel and closes it.
func feedJobs(rows [][]string) <-chan []string {
	jobs := make(chan []string, len(rows))
	for _, r := range rows {
		jobs <- r
	}
	close(jobs)
	return jobs
}

func TestRun_SingleCampaignMultipleRows(t *testing.T) {
	jobs := feedJobs([][]string{
		{"CMP001", "1000", "50", "100.50", "5"},
		{"CMP001", "500", "20", "40.00", "2"},
	})

	a := New(Config{Workers: 2})
	stats, err := a.Run(context.Background(), jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s, ok := stats["CMP001"]
	if !ok {
		t.Fatal("CMP001 not found in stats")
	}
	if s.TotalImpressions != 1500 {
		t.Errorf("TotalImpressions = %d, want 1500", s.TotalImpressions)
	}
	if s.TotalClicks != 70 {
		t.Errorf("TotalClicks = %d, want 70", s.TotalClicks)
	}
	if s.TotalConversions != 7 {
		t.Errorf("TotalConversions = %d, want 7", s.TotalConversions)
	}
	// spend: 100.50 + 40.00 = 140.50 (use tolerance for float)
	if abs(s.TotalSpend-140.50) > 0.001 {
		t.Errorf("TotalSpend = %f, want 140.50", s.TotalSpend)
	}
}

func TestRun_MultipleCampaigns(t *testing.T) {
	jobs := feedJobs([][]string{
		{"CMP001", "1000", "50", "100.00", "5"},
		{"CMP002", "2000", "60", "120.00", "0"},
		{"CMP001", "500", "25", "50.00", "3"},
	})

	a := New(Config{Workers: 2})
	stats, err := a.Run(context.Background(), jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("len(stats) = %d, want 2", len(stats))
	}
	if stats["CMP001"].TotalImpressions != 1500 {
		t.Errorf("CMP001 TotalImpressions = %d, want 1500", stats["CMP001"].TotalImpressions)
	}
	if stats["CMP002"].TotalConversions != 0 {
		t.Errorf("CMP002 TotalConversions = %d, want 0", stats["CMP002"].TotalConversions)
	}
}

func TestRun_AllBadRows(t *testing.T) {
	jobs := feedJobs([][]string{
		{"CMP001", "not-a-number", "50", "100.00", "5"},
		{"CMP002", "2000", "not-a-number", "120.00", "0"},
	})

	a := New(Config{Workers: 2})
	stats, err := a.Run(context.Background(), jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty stats map for all-bad rows, got %d entries", len(stats))
	}
}

func TestRun_EmptyJobs(t *testing.T) {
	jobs := feedJobs(nil)
	a := New(Config{Workers: 2})
	stats, err := a.Run(context.Background(), jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty stats, got %d", len(stats))
	}
}

func TestParseRow_Valid(t *testing.T) {
	row := []string{"CMP001", "1000", "50", "100.50", "5"}
	rec, err := parseRow(row)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.CampaignID != "CMP001" {
		t.Errorf("CampaignID = %q, want CMP001", rec.CampaignID)
	}
	if rec.Impressions != 1000 {
		t.Errorf("Impressions = %d, want 1000", rec.Impressions)
	}
}

func TestParseRow_InvalidImpressions(t *testing.T) {
	_, err := parseRow([]string{"CMP001", "abc", "50", "100.50", "5"})
	if err == nil {
		t.Fatal("expected error for invalid impressions")
	}
}

func TestParseRow_InvalidSpend(t *testing.T) {
	_, err := parseRow([]string{"CMP001", "1000", "50", "not-float", "5"})
	if err == nil {
		t.Fatal("expected error for invalid spend")
	}
}

func TestParseRow_WrongFieldCount(t *testing.T) {
	_, err := parseRow([]string{"CMP001", "1000"})
	if err == nil {
		t.Fatal("expected error for wrong field count")
	}
}

func TestParseRow_NegativeValues(t *testing.T) {
	cases := []struct {
		name string
		row  []string
	}{
		{"negative impressions", []string{"CMP001", "-1000", "50", "100.00", "5"}},
		{"negative clicks", []string{"CMP001", "1000", "-50", "100.00", "5"}},
		{"negative spend", []string{"CMP001", "1000", "50", "-100.00", "5"}},
		{"negative conversions", []string{"CMP001", "1000", "50", "100.00", "-5"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseRow(c.row)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", c.name)
			}
		})
	}
}

func TestRun_WorkersZero_ReturnsError(t *testing.T) {
	jobs := feedJobs(nil)
	_, err := New(Config{Workers: 0}).Run(context.Background(), jobs)
	if err == nil {
		t.Fatal("expected error for Workers: 0, got nil")
	}
}

func TestRun_WorkersNegative_ReturnsError(t *testing.T) {
	jobs := feedJobs(nil)
	_, err := New(Config{Workers: -1}).Run(context.Background(), jobs)
	if err == nil {
		t.Fatal("expected error for Workers: -1, got nil")
	}
}

func TestRun_PreCancelledContext_DoesNotHang(t *testing.T) {
	jobs := feedJobs([][]string{
		{"CMP001", "1000", "50", "100.00", "5"},
		{"CMP002", "2000", "60", "120.00", "10"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run is called

	// Must return promptly without hanging — does not panic or deadlock.
	done := make(chan struct{})
	go func() {
		defer close(done)
		New(Config{Workers: 2}).Run(ctx, jobs) //nolint:errcheck
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after pre-cancelled context")
	}
}

func TestAggregate_ZeroConversions(t *testing.T) {
	results := make(chan models.Record, 1)
	results <- models.Record{CampaignID: "CMP001", Impressions: 1000, Clicks: 0, Spend: 0, Conversions: 0}
	close(results)

	stats := aggregate(results)
	if stats["CMP001"].TotalConversions != 0 {
		t.Errorf("expected zero conversions")
	}
}

// TestWorker_ExitsOnCancellation_WhenResultsBlocked proves that a worker does
// not hang indefinitely on "results <- rec" when the results channel is full
// and the context is cancelled.
//
// Synchronisation strategy: both jobs and results are unbuffered.
// The send `jobs <- row` blocks until the worker receives it — this is a
// happens-before guarantee that the worker holds the record and is about to
// reach `results <- rec` when we call cancel(). No time.Sleep needed.
func TestWorker_ExitsOnCancellation_WhenResultsBlocked(t *testing.T) {
	results := make(chan models.Record) // unbuffered — blocks worker on send
	jobs := make(chan []string)         // unbuffered — synchronises with worker receive

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		worker(ctx, jobs, results)
	}()

	// Blocks until the worker receives the row. After this returns, the worker
	// has the record and is about to block on `results <- rec`.
	jobs <- []string{"CMP001", "1000", "50", "100.00", "5"}

	// Cancel the context — the inner select must unblock the worker.
	cancel()

	select {
	case <-done:
		// passed: worker exited promptly after cancellation
	case <-time.After(time.Second):
		t.Fatal("worker blocked on results send and did not exit after context cancellation")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
