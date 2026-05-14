package parse

import "testing"

func TestParseRowValid(t *testing.T) {
	row, err := ParseRow([]byte("CMP001,2025-01-01,12000,300,45.50,12"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(row.CampaignID) != "CMP001" {
		t.Errorf("CampaignID = %q, want CMP001", row.CampaignID)
	}
	if row.Impressions != 12000 || row.Clicks != 300 || row.Conversions != 12 {
		t.Errorf("unexpected counts: %+v", row)
	}
	if row.Spend != 45.50 {
		t.Errorf("Spend = %v, want 45.50", row.Spend)
	}
}

func TestParseRowZeroValuesAreValid(t *testing.T) {
	row, err := ParseRow([]byte("CMP009,2025-01-01,0,0,0,0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.Impressions != 0 || row.Clicks != 0 || row.Conversions != 0 || row.Spend != 0 {
		t.Errorf("unexpected row: %+v", row)
	}
}

func TestParseRowErrors(t *testing.T) {
	cases := map[string]string{
		"too few fields":    "CMP001,2025-01-01,12000,300,45.50",
		"too many fields":   "CMP001,2025-01-01,12000,300,45.50,12,extra",
		"empty campaign id": ",2025-01-01,12000,300,45.50,12",
		"non-numeric int":   "CMP001,2025-01-01,12k,300,45.50,12",
		"negative int":      "CMP001,2025-01-01,-5,300,45.50,12",
		"empty int":         "CMP001,2025-01-01,,300,45.50,12",
		"non-numeric spend": "CMP001,2025-01-01,12000,300,USD,12",
		"negative spend":    "CMP001,2025-01-01,12000,300,-1.0,12",
		"empty line":        "",
	}
	for name, line := range cases {
		if _, err := ParseRow([]byte(line)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
