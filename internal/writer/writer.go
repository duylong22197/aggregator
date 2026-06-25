package writer

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/duylong22197/aggregator/internal/models"
)

const (
	ctrFilename = "top10_ctr.csv"
	cpaFilename = "top10_cpa.csv"
)

var csvHeader = []string{
	"campaign_id",
	"total_impressions",
	"total_clicks",
	"total_spend",
	"total_conversions",
	"CTR",
	"CPA",
}

// Config holds the output directory path.
type Config struct {
	OutputDir string
}

// Writer writes ranking result CSV files to an output directory.
type Writer struct {
	cfg Config
}

// New creates a Writer with the given Config.
func New(cfg Config) *Writer {
	return &Writer{cfg: cfg}
}

// WriteCTR writes top10_ctr.csv. CPA is formatted as "null" for campaigns
// with zero conversions (the CTR ranking may include them).
func (w *Writer) WriteCTR(results []models.CampaignResult) error {
	return w.write(ctrFilename, results)
}

// WriteCPA writes top10_cpa.csv. All results are expected to have non-nil CPA.
func (w *Writer) WriteCPA(results []models.CampaignResult) error {
	return w.write(cpaFilename, results)
}

// CTRPath returns the full path of the CTR output file.
func (w *Writer) CTRPath() string { return filepath.Join(w.cfg.OutputDir, ctrFilename) }

// CPAPath returns the full path of the CPA output file.
func (w *Writer) CPAPath() string { return filepath.Join(w.cfg.OutputDir, cpaFilename) }

func (w *Writer) write(filename string, results []models.CampaignResult) error {
	if err := os.MkdirAll(w.cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory %q: %w", w.cfg.OutputDir, err)
	}

	path := filepath.Join(w.cfg.OutputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", path, err)
	}
	defer f.Close()

	cw := csv.NewWriter(f)

	if err := cw.Write(csvHeader); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, r := range results {
		row := []string{
			r.CampaignID,
			strconv.FormatInt(r.TotalImpressions, 10),
			strconv.FormatInt(r.TotalClicks, 10),
			strconv.FormatFloat(r.TotalSpend, 'f', 2, 64),
			strconv.FormatInt(r.TotalConversions, 10),
			strconv.FormatFloat(r.CTR, 'f', 4, 64),
			formatCPA(r.CPA),
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("write row for %s: %w", r.CampaignID, err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("flush csv writer: %w", err)
	}
	return nil
}

// formatCPA returns the CPA as a 2-decimal string, or "null" when nil.
func formatCPA(cpa *float64) string {
	if cpa == nil {
		return "null"
	}
	return strconv.FormatFloat(*cpa, 'f', 2, 64)
}
