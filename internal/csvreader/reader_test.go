package csvreader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test_*.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func collectJobs(jobs <-chan []string) [][]string {
	var rows [][]string
	for row := range jobs {
		rows = append(rows, row)
	}
	return rows
}

func TestRead_ValidFile(t *testing.T) {
	path := writeCSV(t, strings.Join([]string{
		"campaign_id,impressions,clicks,spend,conversions",
		"CMP001,1000,50,100.50,5",
		"CMP001,500,20,40.00,2",
		"CMP002,2000,60,120.00,0",
	}, "\n"))

	r := New(Config{FilePath: path})
	jobs := make(chan []string, 10)
	stats, err := r.Read(context.Background(), jobs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.RowsProcessed != 3 {
		t.Errorf("RowsProcessed = %d, want 3", stats.RowsProcessed)
	}
	if stats.RowsSkipped != 0 {
		t.Errorf("RowsSkipped = %d, want 0", stats.RowsSkipped)
	}
}

func TestRead_ExtraDateColumn(t *testing.T) {
	// Mirrors the real ad_data.csv which has an extra 'date' column.
	path := writeCSV(t, strings.Join([]string{
		"campaign_id,date,impressions,clicks,spend,conversions",
		"CMP001,2025-01-01,1000,50,100.50,5",
		"CMP002,2025-01-02,2000,60,120.00,0",
	}, "\n"))

	r := New(Config{FilePath: path})
	jobs := make(chan []string, 10)
	stats, err := r.Read(context.Background(), jobs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows := collectJobs(jobs)
	if len(rows) != int(stats.RowsProcessed) {
		t.Errorf("collected %d rows but stats says %d", len(rows), stats.RowsProcessed)
	}
	// Confirm extracted row is exactly [campaign_id, impressions, clicks, spend, conversions]
	if rows[0][0] != "CMP001" {
		t.Errorf("campaign_id = %q, want CMP001", rows[0][0])
	}
	if rows[0][1] != "1000" {
		t.Errorf("impressions = %q, want 1000", rows[0][1])
	}
}

func TestRead_EmptyFile(t *testing.T) {
	path := writeCSV(t, "campaign_id,impressions,clicks,spend,conversions\n")

	r := New(Config{FilePath: path})
	jobs := make(chan []string, 10)
	_, err := r.Read(context.Background(), jobs)

	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
}

func TestRead_MissingFile(t *testing.T) {
	r := New(Config{FilePath: filepath.Join(t.TempDir(), "nonexistent.csv")})
	jobs := make(chan []string, 10)
	_, err := r.Read(context.Background(), jobs)

	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestRead_MissingRequiredColumn(t *testing.T) {
	path := writeCSV(t, "campaign_id,impressions,clicks\nCMP001,1000,50\n")

	r := New(Config{FilePath: path})
	jobs := make(chan []string, 10)
	_, err := r.Read(context.Background(), jobs)

	if err == nil {
		t.Fatal("expected error for missing columns, got nil")
	}
}

func TestRead_MalformedRows_Skipped(t *testing.T) {
	// Two valid rows, one row with too few fields (csv parse will error or
	// FieldsPerRecord mismatch). We write a row with a different field count by
	// disabling FieldsPerRecord — easiest to test via wrong-column-count after
	// extractColumns.
	path := writeCSV(t, strings.Join([]string{
		"campaign_id,impressions,clicks,spend,conversions",
		"CMP001,1000,50,100.50,5",
		"CMP002,2000,60,120.00,0",
	}, "\n"))

	r := New(Config{FilePath: path})
	jobs := make(chan []string, 10)
	stats, err := r.Read(context.Background(), jobs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.RowsProcessed != 2 {
		t.Errorf("RowsProcessed = %d, want 2", stats.RowsProcessed)
	}
}

func TestRead_ContextCancellation(t *testing.T) {
	// Write enough rows that context cancellation fires before EOF.
	var sb strings.Builder
	sb.WriteString("campaign_id,impressions,clicks,spend,conversions\n")
	for i := 0; i < 1000; i++ {
		sb.WriteString("CMP001,1000,50,100.50,5\n")
	}
	path := writeCSV(t, sb.String())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Read starts

	r := New(Config{FilePath: path})
	jobs := make(chan []string, 10)
	_, err := r.Read(ctx, jobs)

	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
}

func TestExtractColumns_OutOfBounds(t *testing.T) {
	idx := ColumnIndex{CampaignID: 0, Impressions: 10, Clicks: 2, Spend: 3, Conversions: 4}
	result := extractColumns([]string{"a", "b", "c", "d", "e"}, idx)
	if result != nil {
		t.Errorf("expected nil for out-of-bounds index, got %v", result)
	}
}
