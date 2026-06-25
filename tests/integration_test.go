package tests

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/duylong22197/aggregator/internal/aggregator"
	"github.com/duylong22197/aggregator/internal/csvreader"
	"github.com/duylong22197/aggregator/internal/ranking"
	"github.com/duylong22197/aggregator/internal/writer"
)

// runPipeline executes the full aggregation pipeline on csvContent and returns
// the parsed rows from the two output CSVs.
func runPipeline(t *testing.T, csvContent string) (ctrRows [][]string, cpaRows [][]string) {
	t.Helper()

	// Write fixture CSV.
	inputFile, err := os.CreateTemp(t.TempDir(), "input_*.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inputFile.WriteString(csvContent); err != nil {
		t.Fatal(err)
	}
	inputFile.Close()

	outputDir := t.TempDir()

	// Stage 1: stream.
	jobs := make(chan []string, 100)
	r := csvreader.New(csvreader.Config{FilePath: inputFile.Name()})

	readerDone := make(chan error, 1)
	go func() {
		_, err := r.Read(context.Background(), jobs)
		readerDone <- err
	}()

	// Stage 2: aggregate.
	agg := aggregator.New(aggregator.Config{Workers: 2})
	stats, err := agg.Run(context.Background(), jobs)
	if err != nil {
		t.Fatalf("aggregation failed: %v", err)
	}
	if err := <-readerDone; err != nil {
		t.Fatalf("reader failed: %v", err)
	}

	// Stage 3: rank.
	topCTR := ranking.TopCTR(stats, 10)
	topCPA := ranking.TopCPA(stats, 10)

	// Stage 4: write.
	w := writer.New(writer.Config{OutputDir: outputDir})
	if err := w.WriteCTR(topCTR); err != nil {
		t.Fatalf("WriteCTR: %v", err)
	}
	if err := w.WriteCPA(topCPA); err != nil {
		t.Fatalf("WriteCPA: %v", err)
	}

	return readCSVFile(t, filepath.Join(outputDir, "top10_ctr.csv")),
		readCSVFile(t, filepath.Join(outputDir, "top10_cpa.csv"))
}

func readCSVFile(t *testing.T, path string) [][]string {
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

// TestIntegration_EndToEnd verifies the full pipeline with deterministic input.
func TestIntegration_EndToEnd(t *testing.T) {
	// Known data:
	//   CMP001: impressions=1500, clicks=150, spend=150.00, conversions=15 → CTR=0.1000, CPA=10.00
	//   CMP002: impressions=1000, clicks=20,  spend=200.00, conversions=10 → CTR=0.0200, CPA=20.00
	//   CMP003: impressions=2000, clicks=300, spend=600.00, conversions=0  → CTR=0.1500, CPA=null
	csvContent := strings.Join([]string{
		"campaign_id,impressions,clicks,spend,conversions",
		"CMP001,1000,100,100.00,10",
		"CMP001,500,50,50.00,5",
		"CMP002,1000,20,200.00,10",
		"CMP003,2000,300,600.00,0",
	}, "\n")

	ctrRows, cpaRows := runPipeline(t, csvContent)

	// CTR file: header + 3 campaigns
	if len(ctrRows) < 2 {
		t.Fatalf("CTR file has too few rows: %d", len(ctrRows))
	}
	// Header check
	if ctrRows[0][0] != "campaign_id" {
		t.Errorf("CTR header[0] = %q, want campaign_id", ctrRows[0][0])
	}
	// Highest CTR is CMP003 (0.1500)
	if ctrRows[1][0] != "CMP003" {
		t.Errorf("CTR top[0] = %q, want CMP003", ctrRows[1][0])
	}
	// CMP003 has null CPA
	if ctrRows[1][6] != "null" {
		t.Errorf("CMP003 CPA = %q, want null", ctrRows[1][6])
	}

	// CPA file: header + 2 campaigns (CMP003 excluded)
	if len(cpaRows) < 3 {
		t.Fatalf("CPA file has too few rows: %d", len(cpaRows))
	}
	// Lowest CPA is CMP001 (10.00)
	if cpaRows[1][0] != "CMP001" {
		t.Errorf("CPA top[0] = %q, want CMP001", cpaRows[1][0])
	}
	if cpaRows[1][6] != "10.00" {
		t.Errorf("CMP001 CPA = %q, want 10.00", cpaRows[1][6])
	}
}

// TestIntegration_ExtraColumnInCSV verifies the pipeline handles a CSV with
// columns beyond the required set (e.g. the real ad_data.csv has a 'date' column).
func TestIntegration_ExtraColumnInCSV(t *testing.T) {
	csvContent := strings.Join([]string{
		"campaign_id,date,impressions,clicks,spend,conversions",
		"CMP001,2025-01-01,2000,100,500.00,50",
		"CMP002,2025-01-01,1000,10,100.00,5",
	}, "\n")

	ctrRows, _ := runPipeline(t, csvContent)

	// CMP001: CTR = 100/2000 = 0.05
	// CMP002: CTR = 10/1000  = 0.01
	// Top CTR should be CMP001
	if len(ctrRows) < 2 {
		t.Fatalf("CTR file too short: %d rows", len(ctrRows))
	}
	if ctrRows[1][0] != "CMP001" {
		t.Errorf("CTR top[0] = %q, want CMP001", ctrRows[1][0])
	}
}

// TestIntegration_AllZeroConversions verifies CPA file is empty when no
// campaign has conversions.
func TestIntegration_AllZeroConversions(t *testing.T) {
	csvContent := strings.Join([]string{
		"campaign_id,impressions,clicks,spend,conversions",
		"CMP001,1000,50,100.00,0",
		"CMP002,2000,60,120.00,0",
	}, "\n")

	_, cpaRows := runPipeline(t, csvContent)

	// Only header row
	if len(cpaRows) != 1 {
		t.Errorf("CPA file expected header only, got %d rows", len(cpaRows))
	}
}

// TestIntegration_MalformedRowsSkipped verifies that rows with invalid numeric
// values are skipped without aborting the pipeline.
func TestIntegration_MalformedRowsSkipped(t *testing.T) {
	csvContent := strings.Join([]string{
		"campaign_id,impressions,clicks,spend,conversions",
		"CMP001,not-a-number,50,100.00,5",
		"CMP002,2000,60,120.00,0",
		"CMP003,1000,50,100.00,10",
	}, "\n")

	ctrRows, _ := runPipeline(t, csvContent)

	// CMP001 skipped; CMP002 and CMP003 present
	campaignIDs := make(map[string]bool)
	for _, row := range ctrRows[1:] { // skip header
		campaignIDs[row[0]] = true
	}
	if campaignIDs["CMP001"] {
		t.Error("CMP001 should have been skipped due to invalid impressions")
	}
	if !campaignIDs["CMP002"] {
		t.Error("CMP002 should be present")
	}
	if !campaignIDs["CMP003"] {
		t.Error("CMP003 should be present")
	}
}
