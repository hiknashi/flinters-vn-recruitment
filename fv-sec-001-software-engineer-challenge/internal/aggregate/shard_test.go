package aggregate

import (
	"strconv"
	"sync"
	"testing"

	"adperf/internal/parse"
)

func row(id string, impressions, clicks, conversions int64, spend float64) parse.Row {
	return parse.Row{
		CampaignID:  []byte(id),
		Impressions: impressions,
		Clicks:      clicks,
		Conversions: conversions,
		Spend:       spend,
	}
}

func byID(stats []*Stats) map[string]*Stats {
	m := make(map[string]*Stats, len(stats))
	for _, s := range stats {
		m[s.CampaignID] = s
	}
	return m
}

func TestCombinerAggregates(t *testing.T) {
	agg := NewShardedAggregator(16)
	c := agg.NewCombiner()
	c.Add(row("A", 100, 10, 2, 5))
	c.Add(row("A", 50, 5, 0, 5))
	c.Add(row("B", 10, 1, 1, 1))
	c.Flush()

	got := byID(agg.collect(nil))
	if len(got) != 2 {
		t.Fatalf("got %d campaigns, want 2", len(got))
	}
	if a := got["A"]; a.Impressions != 150 || a.Clicks != 15 || a.Conversions != 2 || a.Spend != 10 {
		t.Fatalf("campaign A: %+v", a)
	}
	if b := got["B"]; b.Impressions != 10 {
		t.Fatalf("campaign B: %+v", b)
	}
}

// TestCombinerFlushesAcrossThreshold exercises the periodic-flush path: with a
// tiny threshold the combiner flushes many times, and the totals must still be
// correct after every key has been merged into the shared aggregator.
func TestCombinerFlushesAcrossThreshold(t *testing.T) {
	agg := NewShardedAggregator(64)
	c := agg.NewCombiner()
	c.threshold = 8

	const campaigns = 100
	for i := 0; i < campaigns; i++ {
		id := "CMP" + strconv.Itoa(i)
		c.Add(row(id, 1, 1, 1, 1))
		c.Add(row(id, 1, 1, 1, 1))
	}
	c.Flush()

	all := agg.collect(nil)
	if len(all) != campaigns {
		t.Fatalf("got %d campaigns, want %d", len(all), campaigns)
	}
	for _, s := range all {
		if s.Impressions != 2 || s.Conversions != 2 {
			t.Fatalf("%s: %+v", s.CampaignID, s)
		}
	}
}

// TestShardedAggregatorConcurrent feeds the aggregator from many goroutines to
// confirm shard locking keeps the totals consistent.
func TestShardedAggregatorConcurrent(t *testing.T) {
	agg := NewShardedAggregator(64)
	const (
		workers       = 8
		rowsPerWorker = 5000
		campaigns     = 50
	)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := agg.NewCombiner()
			c.threshold = 32
			for i := 0; i < rowsPerWorker; i++ {
				c.Add(row("CMP"+strconv.Itoa(i%campaigns), 1, 1, 1, 1))
			}
			c.Flush()
		}()
	}
	wg.Wait()

	all := agg.collect(nil)
	if len(all) != campaigns {
		t.Fatalf("got %d campaigns, want %d", len(all), campaigns)
	}
	var total int64
	for _, s := range all {
		total += s.Impressions
	}
	if want := int64(workers * rowsPerWorker); total != want {
		t.Fatalf("total impressions = %d, want %d", total, want)
	}
}
