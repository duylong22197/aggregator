package ranking

import (
	"testing"

	"github.com/duylong22197/aggregator/internal/models"
)

func TestTopCTR_Ordering(t *testing.T) {
	// CTR: A=0.10, B=0.05, C=0.20, D=0.01
	stats := map[string]*models.CampaignStats{
		"A": {CampaignID: "A", TotalImpressions: 1000, TotalClicks: 100},
		"B": {CampaignID: "B", TotalImpressions: 1000, TotalClicks: 50},
		"C": {CampaignID: "C", TotalImpressions: 1000, TotalClicks: 200},
		"D": {CampaignID: "D", TotalImpressions: 1000, TotalClicks: 10},
	}

	top := TopCTR(stats, 3)

	if len(top) != 3 {
		t.Fatalf("expected 3 results, got %d", len(top))
	}
	if top[0].CampaignID != "C" {
		t.Errorf("top[0] = %s, want C", top[0].CampaignID)
	}
	if top[1].CampaignID != "A" {
		t.Errorf("top[1] = %s, want A", top[1].CampaignID)
	}
	if top[2].CampaignID != "B" {
		t.Errorf("top[2] = %s, want B", top[2].CampaignID)
	}
}

func TestTopCTR_ZeroImpressions(t *testing.T) {
	stats := map[string]*models.CampaignStats{
		"A": {CampaignID: "A", TotalImpressions: 0, TotalClicks: 100},
		"B": {CampaignID: "B", TotalImpressions: 1000, TotalClicks: 50},
	}

	top := TopCTR(stats, 10)
	// A should have CTR=0, B CTR=0.05; B must rank first
	if top[0].CampaignID != "B" {
		t.Errorf("expected B first, got %s", top[0].CampaignID)
	}
	var aResult models.CampaignResult
	for _, r := range top {
		if r.CampaignID == "A" {
			aResult = r
		}
	}
	if aResult.CTR != 0 {
		t.Errorf("CTR for zero-impression campaign = %f, want 0", aResult.CTR)
	}
}

func TestTopCTR_FewerThanN(t *testing.T) {
	stats := map[string]*models.CampaignStats{
		"A": {CampaignID: "A", TotalImpressions: 1000, TotalClicks: 100},
	}
	top := TopCTR(stats, 10)
	if len(top) != 1 {
		t.Errorf("expected 1 result, got %d", len(top))
	}
}

func TestTopCTR_Empty(t *testing.T) {
	top := TopCTR(map[string]*models.CampaignStats{}, 10)
	if len(top) != 0 {
		t.Errorf("expected 0 results, got %d", len(top))
	}
}

func TestTopCPA_Ordering(t *testing.T) {
	// CPA: A=10, B=5, C=20, D=1 (spend/conversions)
	stats := map[string]*models.CampaignStats{
		"A": {CampaignID: "A", TotalSpend: 100, TotalConversions: 10, TotalImpressions: 1000, TotalClicks: 50},
		"B": {CampaignID: "B", TotalSpend: 50, TotalConversions: 10, TotalImpressions: 1000, TotalClicks: 50},
		"C": {CampaignID: "C", TotalSpend: 200, TotalConversions: 10, TotalImpressions: 1000, TotalClicks: 50},
		"D": {CampaignID: "D", TotalSpend: 10, TotalConversions: 10, TotalImpressions: 1000, TotalClicks: 50},
	}

	top := TopCPA(stats, 3)

	if len(top) != 3 {
		t.Fatalf("expected 3 results, got %d", len(top))
	}
	// lowest CPA first: D(1) < B(5) < A(10)
	if top[0].CampaignID != "D" {
		t.Errorf("top[0] = %s, want D (CPA=1)", top[0].CampaignID)
	}
	if top[1].CampaignID != "B" {
		t.Errorf("top[1] = %s, want B (CPA=5)", top[1].CampaignID)
	}
	if top[2].CampaignID != "A" {
		t.Errorf("top[2] = %s, want A (CPA=10)", top[2].CampaignID)
	}
}

func TestTopCPA_ExcludesZeroConversions(t *testing.T) {
	stats := map[string]*models.CampaignStats{
		"A": {CampaignID: "A", TotalSpend: 100, TotalConversions: 0},
		"B": {CampaignID: "B", TotalSpend: 50, TotalConversions: 10, TotalImpressions: 1000, TotalClicks: 50},
	}

	top := TopCPA(stats, 10)
	if len(top) != 1 {
		t.Errorf("expected 1 result (A excluded), got %d", len(top))
	}
	if top[0].CampaignID != "B" {
		t.Errorf("expected B, got %s", top[0].CampaignID)
	}
}

func TestTopCPA_AllZeroConversions(t *testing.T) {
	stats := map[string]*models.CampaignStats{
		"A": {CampaignID: "A", TotalSpend: 100, TotalConversions: 0},
	}
	top := TopCPA(stats, 10)
	if len(top) != 0 {
		t.Errorf("expected 0 results, got %d", len(top))
	}
}

func TestTopCPA_CPAValueNonNil(t *testing.T) {
	stats := map[string]*models.CampaignStats{
		"A": {CampaignID: "A", TotalSpend: 100, TotalConversions: 10, TotalImpressions: 1000, TotalClicks: 50},
	}
	top := TopCPA(stats, 10)
	if top[0].CPA == nil {
		t.Error("CPA should not be nil for campaign with conversions")
	}
	if *top[0].CPA != 10 {
		t.Errorf("CPA = %f, want 10", *top[0].CPA)
	}
}

func TestComputeCTR_ZeroImpressions(t *testing.T) {
	s := &models.CampaignStats{TotalImpressions: 0, TotalClicks: 100}
	if computeCTR(s) != 0 {
		t.Error("expected CTR=0 for zero impressions")
	}
}
