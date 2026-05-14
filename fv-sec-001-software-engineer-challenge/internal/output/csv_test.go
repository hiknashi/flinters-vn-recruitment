package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adperf/internal/aggregate"
)

func TestWriteResults(t *testing.T) {
	dir := t.TempDir()
	byCTR := []*aggregate.Stats{
		{CampaignID: "A", Impressions: 1000, Clicks: 50, Spend: 100, Conversions: 5},
		{CampaignID: "B", Impressions: 2000, Clicks: 40, Spend: 80, Conversions: 0},
	}
	byCPA := []*aggregate.Stats{
		{CampaignID: "A", Impressions: 1000, Clicks: 50, Spend: 100, Conversions: 5},
	}
	if err := WriteResults(dir, 10, byCTR, byCPA); err != nil {
		t.Fatalf("WriteResults: %v", err)
	}

	lines := readLines(t, filepath.Join(dir, "top10_ctr.csv"))
	wantHeader := "campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA"
	if lines[0] != wantHeader {
		t.Fatalf("header = %q, want %q", lines[0], wantHeader)
	}
	if lines[1] != "A,1000,50,100.00,5,0.0500,20.00" {
		t.Errorf("row A = %q", lines[1])
	}
	// Campaign B has no conversions: CPA must be an empty (null) field.
	if lines[2] != "B,2000,40,80.00,0,0.0200," {
		t.Errorf("row B = %q, want trailing empty CPA", lines[2])
	}

	if _, err := os.Stat(filepath.Join(dir, "top10_cpa.csv")); err != nil {
		t.Fatalf("cpa file missing: %v", err)
	}
}

func TestWriteResultsCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "results")
	if err := WriteResults(dir, 5, nil, nil); err != nil {
		t.Fatalf("WriteResults: %v", err)
	}
	for _, name := range []string{"top5_ctr.csv", "top5_cpa.csv"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not created: %v", name, err)
		}
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}
