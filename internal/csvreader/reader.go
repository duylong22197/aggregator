package csvreader

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
)

const (
	bufferSize    = 4 * 1024 * 1024 // 4MB read buffer
	progressEvery = 1_000_000
)

// requiredHeaders are the column names that must be present (order-independent).
var requiredHeaders = []string{"campaign_id", "impressions", "clicks", "spend", "conversions"}

// ColumnIndex maps each required field to its CSV column position.
type ColumnIndex struct {
	CampaignID  int
	Impressions int
	Clicks      int
	Spend       int
	Conversions int
}

// ReadStats summarises the outcome of a Read call.
type ReadStats struct {
	RowsProcessed int64
	RowsSkipped   int64
}

// Config holds the parameters for a Reader.
type Config struct {
	FilePath string
}

// Reader streams rows from a CSV file into a jobs channel.
type Reader struct {
	cfg Config
}

// New constructs a Reader with the given Config.
func New(cfg Config) *Reader {
	return &Reader{cfg: cfg}
}

// Read opens the file, validates the header, and streams raw []string rows into
// jobs. It closes jobs when the file is exhausted or an unrecoverable error
// occurs. Malformed rows are skipped with a warning log.
func (r *Reader) Read(ctx context.Context, jobs chan<- []string) (ReadStats, error) {
	defer close(jobs)

	f, err := os.Open(r.cfg.FilePath)
	if err != nil {
		return ReadStats{}, fmt.Errorf("open input file: %w", err)
	}
	defer f.Close()

	br := bufio.NewReaderSize(f, bufferSize)
	cr := csv.NewReader(br)
	cr.ReuseRecord = false // each row must be safe to send on a channel

	idx, err := readHeader(cr)
	if err != nil {
		return ReadStats{}, err
	}

	var stats ReadStats
	var totalLines int64 // every data line attempted, including skipped
	for lineNum := int64(2); ; lineNum++ {
		row, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		totalLines++
		if err != nil {
			slog.Warn("skipping unreadable row", "line", lineNum, "error", err)
			stats.RowsSkipped++
			continue
		}

		// Extract only the 5 required columns into a fixed-length slice so
		// downstream workers never need to know about the original column layout.
		extracted := extractColumns(row, idx)
		if extracted == nil {
			slog.Warn("skipping row with out-of-bounds column index", "line", lineNum)
			stats.RowsSkipped++
			continue
		}

		// Use a select so the send is interruptible by context cancellation.
		// Without this guard, workers exiting early (ctx cancelled) leave nobody
		// to drain the jobs channel, causing the reader goroutine to block forever.
		select {
		case jobs <- extracted:
		case <-ctx.Done():
			return stats, ctx.Err()
		}
		stats.RowsProcessed++

		if totalLines%progressEvery == 0 {
			slog.Info("rows read", "total", totalLines, "skipped", stats.RowsSkipped)
		}
	}

	if stats.RowsProcessed == 0 {
		return stats, fmt.Errorf("file contains no data rows")
	}

	return stats, nil
}

// readHeader reads the first row and resolves required column positions.
func readHeader(cr *csv.Reader) (ColumnIndex, error) {
	header, err := cr.Read()
	if err != nil {
		return ColumnIndex{}, fmt.Errorf("read header: %w", err)
	}

	index := make(map[string]int, len(header))
	for i, name := range header {
		index[name] = i
	}

	missing := make([]string, 0)
	for _, col := range requiredHeaders {
		if _, ok := index[col]; !ok {
			missing = append(missing, col)
		}
	}
	if len(missing) > 0 {
		return ColumnIndex{}, fmt.Errorf("missing required columns: %v (got: %v)", missing, header)
	}

	return ColumnIndex{
		CampaignID:  index["campaign_id"],
		Impressions: index["impressions"],
		Clicks:      index["clicks"],
		Spend:       index["spend"],
		Conversions: index["conversions"],
	}, nil
}

// extractColumns builds a 5-element slice [campaign_id, impressions, clicks, spend, conversions]
// from an arbitrary-width row using the resolved column indices.
// Returns nil if any index is out of range.
func extractColumns(row []string, idx ColumnIndex) []string {
	maxIdx := max(idx.CampaignID, idx.Impressions, idx.Clicks, idx.Spend, idx.Conversions)
	if maxIdx >= len(row) {
		return nil
	}
	return []string{
		row[idx.CampaignID],
		row[idx.Impressions],
		row[idx.Clicks],
		row[idx.Spend],
		row[idx.Conversions],
	}
}
