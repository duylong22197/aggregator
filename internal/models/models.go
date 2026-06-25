package models

// Record is a fully-parsed CSV record ready for aggregation.
type Record struct {
	CampaignID  string
	Impressions int64
	Clicks      int64
	Spend       float64
	Conversions int64
}

// CampaignStats holds running totals for a single campaign.
type CampaignStats struct {
	CampaignID       string
	TotalImpressions int64
	TotalClicks      int64
	TotalSpend       float64
	TotalConversions int64
}

// CampaignResult holds computed metrics ready for output.
// CPA is nil when TotalConversions == 0 to prevent division by zero.
type CampaignResult struct {
	CampaignID       string
	TotalImpressions int64
	TotalClicks      int64
	TotalSpend       float64
	TotalConversions int64
	CTR              float64
	CPA              *float64
}
