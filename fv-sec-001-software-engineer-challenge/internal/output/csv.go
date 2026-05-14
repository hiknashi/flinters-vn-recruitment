// Package output writes ranked campaign results as CSV files.
package output

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"adperf/internal/aggregate"
)

// header is the column order shared by both result files.
var header = []string{
	"campaign_id",
	"total_impressions",
	"total_clicks",
	"total_spend",
	"total_conversions",
	"CTR",
	"CPA",
}

// Number-formatting precision for the derived columns.
const (
	spendDecimals = 2
	ctrDecimals   = 4
	cpaDecimals   = 2
)

// WriteResults writes two files into dir: top<n>_ctr.csv and top<n>_cpa.csv.
// The directory is created if it does not exist.
func WriteResults(dir string, n int, byCTR, byCPA []*aggregate.Stats) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := writeCSV(filepath.Join(dir, fmt.Sprintf("top%d_ctr.csv", n)), byCTR); err != nil {
		return err
	}
	return writeCSV(filepath.Join(dir, fmt.Sprintf("top%d_cpa.csv", n)), byCPA)
}

func writeCSV(path string, rows []*aggregate.Stats) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	w := csv.NewWriter(f)
	if err = w.Write(header); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	for _, s := range rows {
		if err = w.Write(record(s)); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	w.Flush()
	return w.Error()
}

// record formats one campaign as a CSV row. CPA is left empty (null) when the
// campaign has no conversions.
func record(s *aggregate.Stats) []string {
	cpa := ""
	if v, ok := s.CPA(); ok {
		cpa = strconv.FormatFloat(v, 'f', cpaDecimals, 64)
	}
	return []string{
		s.CampaignID,
		strconv.FormatInt(s.Impressions, 10),
		strconv.FormatInt(s.Clicks, 10),
		strconv.FormatFloat(s.Spend, 'f', spendDecimals, 64),
		strconv.FormatInt(s.Conversions, 10),
		strconv.FormatFloat(s.CTR(), 'f', ctrDecimals, 64),
		cpa,
	}
}
