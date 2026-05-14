package aggregate

import "testing"

func aggWith(stats ...*Stats) *ShardedAggregator {
	agg := NewShardedAggregator(8)
	for _, s := range stats {
		agg.merge(s.CampaignID, s)
	}
	return agg
}

func ids(stats []*Stats) []string {
	out := make([]string, len(stats))
	for i, s := range stats {
		out[i] = s.CampaignID
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTopByCTR(t *testing.T) {
	agg := aggWith(
		&Stats{CampaignID: "A", Impressions: 1000, Clicks: 50}, // 0.05
		&Stats{CampaignID: "B", Impressions: 1000, Clicks: 10}, // 0.01
		&Stats{CampaignID: "C", Impressions: 1000, Clicks: 30}, // 0.03
		&Stats{CampaignID: "D", Impressions: 1000, Clicks: 50}, // 0.05, ties A
	)
	// Highest CTR first; A before D because ties break on campaign_id ascending.
	want := []string{"A", "D", "C"}
	if got := ids(agg.TopByCTR(3)); !equal(got, want) {
		t.Fatalf("TopByCTR = %v, want %v", got, want)
	}
}

func TestTopByCPAExcludesZeroConversions(t *testing.T) {
	agg := aggWith(
		&Stats{CampaignID: "A", Spend: 100, Conversions: 10}, // CPA 10
		&Stats{CampaignID: "B", Spend: 100, Conversions: 5},  // CPA 20
		&Stats{CampaignID: "C", Spend: 100, Conversions: 0},  // excluded
		&Stats{CampaignID: "D", Spend: 50, Conversions: 10},  // CPA 5
	)
	want := []string{"D", "A", "B"}
	got := ids(agg.TopByCPA(10))
	if !equal(got, want) {
		t.Fatalf("TopByCPA = %v, want %v (zero-conversion campaign excluded)", got, want)
	}
}

func TestTopKRequestedMoreThanAvailable(t *testing.T) {
	agg := aggWith(&Stats{CampaignID: "A", Impressions: 100, Clicks: 1})
	if got := agg.TopByCTR(10); len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
}

func TestTopKZero(t *testing.T) {
	agg := aggWith(&Stats{CampaignID: "A", Impressions: 100, Clicks: 1})
	if got := agg.TopByCTR(0); got != nil {
		t.Fatalf("TopByCTR(0) = %v, want nil", got)
	}
}
