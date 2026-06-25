package ranking

import (
	"container/heap"

	"github.com/duylong22197/aggregator/internal/models"
)

// TopCTR returns up to n campaigns with the highest CTR, sorted descending.
// Campaigns with zero impressions receive CTR = 0 per spec.
func TopCTR(stats map[string]*models.CampaignStats, n int) []models.CampaignResult {
	h := &ctrMinHeap{}
	heap.Init(h)

	for _, s := range stats {
		ctr := computeCTR(s)
		result := buildResult(s, ctr)
		if s.TotalConversions > 0 {
			cpa := computeCPA(s)
			result.CPA = &cpa
		}

		if h.Len() < n {
			heap.Push(h, result)
		} else if h.Len() > 0 && ctr > (*h)[0].CTR {
			heap.Pop(h)
			heap.Push(h, result)
		}
	}

	// Pop from a min-heap yields ascending CTR; filling from the back
	// produces descending order without an extra sort pass.
	out := make([]models.CampaignResult, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(models.CampaignResult)
	}
	return out
}

// TopCPA returns up to n campaigns with the lowest CPA, sorted ascending.
// Campaigns with zero conversions are excluded.
func TopCPA(stats map[string]*models.CampaignStats, n int) []models.CampaignResult {
	h := &cpaMaxHeap{}
	heap.Init(h)

	for _, s := range stats {
		if s.TotalConversions == 0 {
			continue
		}
		cpa := computeCPA(s)
		result := buildResult(s, computeCTR(s))
		result.CPA = &cpa

		if h.Len() < n {
			heap.Push(h, result)
		} else if h.Len() > 0 && cpa < cpaValue((*h)[0]) {
			heap.Pop(h)
			heap.Push(h, result)
		}
	}

	// Pop from a max-heap yields descending CPA; filling from the back
	// produces ascending order without an extra sort pass.
	out := make([]models.CampaignResult, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(models.CampaignResult)
	}
	return out
}

// computeCTR returns clicks/impressions; 0 if impressions == 0.
func computeCTR(s *models.CampaignStats) float64 {
	if s.TotalImpressions == 0 {
		return 0
	}
	return float64(s.TotalClicks) / float64(s.TotalImpressions)
}

// computeCPA returns spend/conversions. Callers must ensure conversions > 0.
func computeCPA(s *models.CampaignStats) float64 {
	return s.TotalSpend / float64(s.TotalConversions)
}

// buildResult constructs a CampaignResult from stats and a pre-computed CTR.
// CPA is left nil; callers that need it set it explicitly.
func buildResult(s *models.CampaignStats, ctr float64) models.CampaignResult {
	return models.CampaignResult{
		CampaignID:       s.CampaignID,
		TotalImpressions: s.TotalImpressions,
		TotalClicks:      s.TotalClicks,
		TotalSpend:       s.TotalSpend,
		TotalConversions: s.TotalConversions,
		CTR:              ctr,
	}
}

// ---------------------------------------------------------------------------
// ctrMinHeap — min-heap ordered by CTR (lowest CTR at root).
// Maintaining a min-heap of size k lets us efficiently discard the worst
// element whenever a better candidate arrives, keeping O(log k) inserts.
// ---------------------------------------------------------------------------

type ctrMinHeap []models.CampaignResult

func (h ctrMinHeap) Len() int           { return len(h) }
func (h ctrMinHeap) Less(i, j int) bool { return h[i].CTR < h[j].CTR }
func (h ctrMinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *ctrMinHeap) Push(x any) { *h = append(*h, x.(models.CampaignResult)) }
func (h *ctrMinHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// ---------------------------------------------------------------------------
// cpaMaxHeap — max-heap ordered by CPA (highest CPA at root).
// Maintaining a max-heap of size k lets us discard the most expensive
// campaign whenever a cheaper one arrives, keeping the k cheapest overall.
// ---------------------------------------------------------------------------

type cpaMaxHeap []models.CampaignResult

func (h cpaMaxHeap) Len() int           { return len(h) }
func (h cpaMaxHeap) Less(i, j int) bool { return cpaValue(h[i]) > cpaValue(h[j]) }
func (h cpaMaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *cpaMaxHeap) Push(x any) { *h = append(*h, x.(models.CampaignResult)) }
func (h *cpaMaxHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// cpaValue safely dereferences a CampaignResult's CPA pointer.
// Callers always set CPA before pushing onto the cpaMaxHeap, so nil is only
// a defensive guard.
func cpaValue(r models.CampaignResult) float64 {
	if r.CPA == nil {
		return 0
	}
	return *r.CPA
}
