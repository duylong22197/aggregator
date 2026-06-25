package benchmarks

import (
	"context"
	"fmt"
	"testing"

	"github.com/duylong22197/aggregator/internal/aggregator"
	"github.com/duylong22197/aggregator/internal/models"
	"github.com/duylong22197/aggregator/internal/ranking"
)

const (
	numCampaigns   = 1_000
	rowsPerBench   = 100_000
	rankingCampaigns = 100_000
)

// generateRows returns pre-parsed [][]string rows for worker/aggregation benchmarks.
func generateRows(nRows, nCampaigns int) [][]string {
	rows := make([][]string, nRows)
	for i := 0; i < nRows; i++ {
		id := fmt.Sprintf("CMP%06d", i%nCampaigns)
		rows[i] = []string{id, "1000", "50", "100.50", "5"}
	}
	return rows
}

// generateStatsMap builds a map of CampaignStats for ranking benchmarks.
func generateStatsMap(n int) map[string]*models.CampaignStats {
	m := make(map[string]*models.CampaignStats, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("CMP%08d", i)
		m[id] = &models.CampaignStats{
			CampaignID:       id,
			TotalImpressions: int64(1000 + i),
			TotalClicks:      int64(50 + i%100),
			TotalSpend:       float64(100 + i),
			TotalConversions: int64(5 + i%20),
		}
	}
	return m
}

// BenchmarkAggregation measures the worker pool + aggregation throughput for
// 100k pre-generated rows across 1k unique campaigns.
func BenchmarkAggregation(b *testing.B) {
	rows := generateRows(rowsPerBench, numCampaigns)

	b.ResetTimer()
	for range b.N {
		jobs := make(chan []string, len(rows))
		for _, r := range rows {
			jobs <- r
		}
		close(jobs)

		agg := aggregator.New(aggregator.Config{Workers: 4})
		_, err := agg.Run(context.Background(), jobs)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(rowsPerBench*b.N)/b.Elapsed().Seconds(), "rows/s")
}

// BenchmarkRankingTopCTR measures TopCTR heap performance over 100k campaigns.
func BenchmarkRankingTopCTR(b *testing.B) {
	stats := generateStatsMap(rankingCampaigns)
	b.ResetTimer()
	for range b.N {
		_ = ranking.TopCTR(stats, 10)
	}
}

// BenchmarkRankingTopCPA measures TopCPA heap performance over 100k campaigns.
func BenchmarkRankingTopCPA(b *testing.B) {
	stats := generateStatsMap(rankingCampaigns)
	b.ResetTimer()
	for range b.N {
		_ = ranking.TopCPA(stats, 10)
	}
}

// BenchmarkParseAndAggregate benchmarks the full parse + aggregate path by
// feeding raw rows through the worker pool.
func BenchmarkParseAndAggregate(b *testing.B) {
	rows := generateRows(rowsPerBench, numCampaigns)

	b.SetBytes(int64(rowsPerBench * 30)) // approx bytes per row
	b.ResetTimer()

	for range b.N {
		jobs := make(chan []string, 5000)
		go func() {
			for _, r := range rows {
				jobs <- r
			}
			close(jobs)
		}()
		agg := aggregator.New(aggregator.Config{Workers: 4})
		_, err := agg.Run(context.Background(), jobs)
		if err != nil {
			b.Fatal(err)
		}
	}
}
