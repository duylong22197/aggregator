package writer

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/duylong22197/aggregator/internal/models"
)

func ptr(f float64) *float64 { return &f }

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return rows
}

func TestWriteCTR_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	w := New(Config{OutputDir: dir})

	cpa := 20.0
	results := []models.CampaignResult{
		{CampaignID: "CMP001", TotalImpressions: 1000, TotalClicks: 50, TotalSpend: 100.0, TotalConversions: 5, CTR: 0.05, CPA: &cpa},
	}

	if err := w.WriteCTR(results); err != nil {
		t.Fatalf("WriteCTR: %v", err)
	}

	rows := readCSV(t, filepath.Join(dir, "top10_ctr.csv"))
	if len(rows) != 2 { // header + 1 data row
		t.Errorf("expected 2 rows (header + 1), got %d", len(rows))
	}
	if rows[0][0] != "campaign_id" {
		t.Errorf("header[0] = %q, want campaign_id", rows[0][0])
	}
	if rows[1][0] != "CMP001" {
		t.Errorf("data[0] = %q, want CMP001", rows[1][0])
	}
	if rows[1][5] != "0.0500" {
		t.Errorf("CTR = %q, want 0.0500", rows[1][5])
	}
	if rows[1][6] != "20.00" {
		t.Errorf("CPA = %q, want 20.00", rows[1][6])
	}
}

func TestWriteCTR_NilCPA_FormatsNull(t *testing.T) {
	dir := t.TempDir()
	w := New(Config{OutputDir: dir})

	results := []models.CampaignResult{
		{CampaignID: "CMP002", TotalImpressions: 2000, TotalClicks: 60, TotalSpend: 120.0, TotalConversions: 0, CTR: 0.03, CPA: nil},
	}

	if err := w.WriteCTR(results); err != nil {
		t.Fatalf("WriteCTR: %v", err)
	}

	rows := readCSV(t, filepath.Join(dir, "top10_ctr.csv"))
	if rows[1][6] != "null" {
		t.Errorf("CPA = %q, want null", rows[1][6])
	}
}

func TestWriteCPA_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	w := New(Config{OutputDir: dir})

	results := []models.CampaignResult{
		{CampaignID: "CMP007", TotalImpressions: 450000, TotalClicks: 13500, TotalSpend: 13500.0, TotalConversions: 1350, CTR: 0.03, CPA: ptr(10.0)},
	}

	if err := w.WriteCPA(results); err != nil {
		t.Fatalf("WriteCPA: %v", err)
	}

	rows := readCSV(t, filepath.Join(dir, "top10_cpa.csv"))
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
	if rows[1][6] != "10.00" {
		t.Errorf("CPA = %q, want 10.00", rows[1][6])
	}
}

func TestWrite_CreatesOutputDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "deep", "nested", "dir")
	w := New(Config{OutputDir: nested})

	if err := w.WriteCTR([]models.CampaignResult{}); err != nil {
		t.Fatalf("WriteCTR: %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Errorf("output dir not created: %v", err)
	}
}

func TestWrite_EmptyResults(t *testing.T) {
	dir := t.TempDir()
	w := New(Config{OutputDir: dir})

	if err := w.WriteCTR(nil); err != nil {
		t.Fatalf("WriteCTR with nil: %v", err)
	}

	rows := readCSV(t, filepath.Join(dir, "top10_ctr.csv"))
	if len(rows) != 1 { // header only
		t.Errorf("expected 1 row (header only), got %d", len(rows))
	}
}

func TestFormatCPA(t *testing.T) {
	cases := []struct {
		input *float64
		want  string
	}{
		{nil, "null"},
		{ptr(10.0), "10.00"},
		{ptr(0.5), "0.50"},
		{ptr(123.456), "123.46"},
	}
	for _, c := range cases {
		got := formatCPA(c.input)
		if got != c.want {
			t.Errorf("formatCPA(%v) = %q, want %q", c.input, got, c.want)
		}
	}
}
