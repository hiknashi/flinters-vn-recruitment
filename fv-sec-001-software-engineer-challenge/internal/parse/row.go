// Package parse turns raw CSV bytes into structured ad-performance rows.
package parse

import (
	"errors"
	"math"
	"strconv"
)

// Row is one parsed ad-performance record.
//
// CampaignID aliases the caller's read buffer and is only valid until the next
// read from that buffer; a consumer that retains it (for example as a map key)
// must copy it first. Returning a borrowed slice instead of a freshly
// allocated string avoids one allocation per input row, which matters when the
// input has tens of millions of rows.
//
// The date column is required to be present but is not retained: aggregation
// is by campaign only.
type Row struct {
	CampaignID  []byte
	Impressions int64
	Clicks      int64
	Spend       float64
	Conversions int64
}

// ErrMalformedRow is returned for any line that does not match the schema:
//
//	campaign_id,date,impressions,clicks,spend,conversions
var ErrMalformedRow = errors.New("malformed row")

const expectedFields = 6

// Field positions within a CSV line.
const (
	fieldCampaignID = iota
	fieldDate
	fieldImpressions
	fieldClicks
	fieldSpend
	fieldConversions
)

// ParseRow parses a single CSV line with the end-of-line bytes already
// stripped. It is deliberately strict: a wrong field count, an empty
// campaign_id, or a non-numeric / negative number all yield ErrMalformedRow so
// the caller can skip and count the bad line rather than corrupt the totals.
func ParseRow(line []byte) (Row, error) {
	var (
		row   Row
		field int
		start int
	)
	for i := 0; i <= len(line); i++ {
		if i < len(line) && line[i] != ',' {
			continue
		}
		col := line[start:i]
		switch field {
		case fieldCampaignID:
			row.CampaignID = col
		case fieldImpressions:
			n, err := parseCount(col)
			if err != nil {
				return Row{}, ErrMalformedRow
			}
			row.Impressions = n
		case fieldClicks:
			n, err := parseCount(col)
			if err != nil {
				return Row{}, ErrMalformedRow
			}
			row.Clicks = n
		case fieldSpend:
			f, err := parseSpend(col)
			if err != nil {
				return Row{}, ErrMalformedRow
			}
			row.Spend = f
		case fieldConversions:
			n, err := parseCount(col)
			if err != nil {
				return Row{}, ErrMalformedRow
			}
			row.Conversions = n
		}
		field++
		start = i + 1
	}
	if field != expectedFields || len(row.CampaignID) == 0 {
		return Row{}, ErrMalformedRow
	}
	return row, nil
}

// parseCount parses a non-negative integer. Impressions, clicks and
// conversions are counts, so a sign or any non-digit byte is malformed.
func parseCount(b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, ErrMalformedRow
	}
	var n int64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, ErrMalformedRow
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

// parseSpend parses a non-negative monetary amount. strconv.ParseFloat is used
// for correctness; the temporary string does not escape, so the conversion is
// stack-allocated.
func parseSpend(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, ErrMalformedRow
	}
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ErrMalformedRow
	}
	return f, nil
}
