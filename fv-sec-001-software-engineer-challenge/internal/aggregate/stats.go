// Package aggregate accumulates ad-performance rows by campaign and ranks the
// results. Memory use is bounded by the number of distinct campaign IDs, not
// by the size of the input.
package aggregate

// Stats holds the running totals for one campaign plus its derived metrics.
type Stats struct {
	CampaignID  string
	Impressions int64
	Clicks      int64
	Spend       float64
	Conversions int64
}

// Add folds a single row's values into the totals.
func (s *Stats) Add(impressions, clicks, conversions int64, spend float64) {
	s.Impressions += impressions
	s.Clicks += clicks
	s.Conversions += conversions
	s.Spend += spend
}

// merge folds the totals of another Stats for the same campaign into s.
func (s *Stats) merge(other *Stats) {
	s.Impressions += other.Impressions
	s.Clicks += other.Clicks
	s.Conversions += other.Conversions
	s.Spend += other.Spend
}

// CTR is total_clicks / total_impressions. It is 0 when there are no
// impressions: a campaign that was never shown cannot be ranked by click rate.
func (s *Stats) CTR() float64 {
	if s.Impressions == 0 {
		return 0
	}
	return float64(s.Clicks) / float64(s.Impressions)
}

// CPA is total_spend / total_conversions. ok is false when there are no
// conversions, in which case CPA is undefined and reported as null.
func (s *Stats) CPA() (value float64, ok bool) {
	if s.Conversions == 0 {
		return 0, false
	}
	return s.Spend / float64(s.Conversions), true
}
