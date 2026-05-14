package aggregate

import "testing"

func TestStatsCTR(t *testing.T) {
	s := &Stats{Impressions: 1000, Clicks: 50}
	if got := s.CTR(); got != 0.05 {
		t.Errorf("CTR = %v, want 0.05", got)
	}
	if got := (&Stats{}).CTR(); got != 0 {
		t.Errorf("CTR with no impressions = %v, want 0", got)
	}
}

func TestStatsCPA(t *testing.T) {
	v, ok := (&Stats{Spend: 100, Conversions: 5}).CPA()
	if !ok || v != 20 {
		t.Errorf("CPA = (%v, %v), want (20, true)", v, ok)
	}
	if _, ok := (&Stats{Spend: 100}).CPA(); ok {
		t.Error("CPA with no conversions: ok = true, want false")
	}
}

func TestStatsAddAndMerge(t *testing.T) {
	s := &Stats{CampaignID: "A"}
	s.Add(10, 1, 2, 1.5)
	s.Add(5, 1, 0, 0.5)
	if s.Impressions != 15 || s.Clicks != 2 || s.Conversions != 2 || s.Spend != 2.0 {
		t.Fatalf("after Add: %+v", s)
	}
	s.merge(&Stats{Impressions: 5, Clicks: 3, Conversions: 1, Spend: 1.0})
	if s.Impressions != 20 || s.Clicks != 5 || s.Conversions != 3 || s.Spend != 3.0 {
		t.Fatalf("after merge: %+v", s)
	}
}
