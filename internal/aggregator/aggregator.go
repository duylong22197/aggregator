package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/duylong22197/aggregator/internal/models"
)

const defaultResultsBuffer = 10_000

// Config controls the worker pool behaviour.
type Config struct {
	Workers int
}

// Aggregator runs a worker pool that parses raw CSV rows and merges them into
// a per-campaign stats map.
type Aggregator struct {
	cfg Config
}

// New creates an Aggregator with the given Config.
func New(cfg Config) *Aggregator {
	return &Aggregator{cfg: cfg}
}

// Run consumes raw []string rows from jobs, parses them in parallel using
// cfg.Workers goroutines, and merges results into a campaign stats map.
// It blocks until jobs is closed and all workers have finished.
func (a *Aggregator) Run(ctx context.Context, jobs <-chan []string) (map[string]*models.CampaignStats, error) {
	results := make(chan models.Record, defaultResultsBuffer)

	// Closer goroutine: wait for all workers to finish then seal the results channel.
	var wg sync.WaitGroup
	wg.Add(a.cfg.Workers)

	for range a.cfg.Workers {
		go func() {
			defer wg.Done()
			worker(ctx, jobs, results)
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Single aggregator goroutine owns the map — no locks needed.
	stats := aggregate(results)
	return stats, nil
}

// worker drains jobs, parses each row, and forwards valid records to results.
// Rows that fail parsing are skipped with a warning.
func worker(ctx context.Context, jobs <-chan []string, results chan<- models.Record) {
	for {
		select {
		case <-ctx.Done():
			return
		case row, ok := <-jobs:
			if !ok {
				return
			}
			rec, err := parseRow(row)
			if err != nil {
				slog.Warn("skipping invalid row", "error", err)
				continue
			}
			results <- rec
		}
	}
}

// aggregate drains the results channel and accumulates per-campaign totals.
func aggregate(results <-chan models.Record) map[string]*models.CampaignStats {
	stats := make(map[string]*models.CampaignStats)
	for rec := range results {
		s, ok := stats[rec.CampaignID]
		if !ok {
			s = &models.CampaignStats{CampaignID: rec.CampaignID}
			stats[rec.CampaignID] = s
		}
		s.TotalImpressions += rec.Impressions
		s.TotalClicks += rec.Clicks
		s.TotalSpend += rec.Spend
		s.TotalConversions += rec.Conversions
	}
	return stats
}

// parseRow converts a 5-element []string slice (campaign_id, impressions,
// clicks, spend, conversions) into a typed Record.
// The slice layout matches what csvreader.extractColumns produces.
func parseRow(row []string) (models.Record, error) {
	if len(row) != 5 {
		return models.Record{}, fmt.Errorf("expected 5 fields, got %d", len(row))
	}

	impressions, err := strconv.ParseInt(row[1], 10, 64)
	if err != nil {
		return models.Record{}, fmt.Errorf("invalid impressions %q: %w", row[1], err)
	}
	if impressions < 0 {
		return models.Record{}, fmt.Errorf("impressions must be non-negative, got %d", impressions)
	}

	clicks, err := strconv.ParseInt(row[2], 10, 64)
	if err != nil {
		return models.Record{}, fmt.Errorf("invalid clicks %q: %w", row[2], err)
	}
	if clicks < 0 {
		return models.Record{}, fmt.Errorf("clicks must be non-negative, got %d", clicks)
	}

	spend, err := strconv.ParseFloat(row[3], 64)
	if err != nil {
		return models.Record{}, fmt.Errorf("invalid spend %q: %w", row[3], err)
	}
	if spend < 0 {
		return models.Record{}, fmt.Errorf("spend must be non-negative, got %f", spend)
	}

	conversions, err := strconv.ParseInt(row[4], 10, 64)
	if err != nil {
		return models.Record{}, fmt.Errorf("invalid conversions %q: %w", row[4], err)
	}
	if conversions < 0 {
		return models.Record{}, fmt.Errorf("conversions must be non-negative, got %d", conversions)
	}

	return models.Record{
		CampaignID:  row[0],
		Impressions: impressions,
		Clicks:      clicks,
		Spend:       spend,
		Conversions: conversions,
	}, nil
}
