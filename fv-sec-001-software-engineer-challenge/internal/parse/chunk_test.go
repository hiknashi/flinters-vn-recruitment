package parse

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSplitRanges(t *testing.T) {
	ranges := SplitRanges(100, 3)
	if len(ranges) != 3 {
		t.Fatalf("got %d ranges, want 3", len(ranges))
	}
	if ranges[0][0] != 0 || ranges[len(ranges)-1][1] != 100 {
		t.Fatalf("ranges do not cover [0,100): %v", ranges)
	}
	for i := 1; i < len(ranges); i++ {
		if ranges[i][0] != ranges[i-1][1] {
			t.Fatalf("ranges not contiguous: %v", ranges)
		}
	}
}

func TestSplitRangesEdgeCases(t *testing.T) {
	if got := SplitRanges(0, 4); len(got) != 1 || got[0] != [2]int64{0, 0} {
		t.Errorf("empty file: got %v", got)
	}
	if got := SplitRanges(3, 10); len(got) != 3 {
		t.Errorf("more workers than bytes: got %v", got)
	}
}

// TestProcessChunkCoversEveryRowOnce verifies the boundary-realignment logic:
// regardless of how the file is split, every data row is parsed exactly once
// and the header is never counted.
func TestProcessChunkCoversEveryRowOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ad.csv")

	const validRows = 1000
	buf := []byte("campaign_id,date,impressions,clicks,spend,conversions\n")
	for i := 0; i < validRows; i++ {
		buf = append(buf, "CMP001,2025-01-01,10,1,1.00,1\n"...)
	}
	buf = append(buf, "CMP001,2025-01-01,bad,1,1.00,1\n"...) // one malformed row
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, workers := range []int{1, 2, 3, 7, 16, 64} {
		var (
			mu       sync.Mutex
			callback int
			res      ChunkResult
			wg       sync.WaitGroup
		)
		for _, rng := range SplitRanges(info.Size(), workers) {
			wg.Add(1)
			go func(s, e int64) {
				defer wg.Done()
				r, err := ProcessChunk(path, s, e, func(Row) {
					mu.Lock()
					callback++
					mu.Unlock()
				})
				if err != nil {
					t.Errorf("workers=%d: ProcessChunk: %v", workers, err)
				}
				mu.Lock()
				res.Parsed += r.Parsed
				res.Skipped += r.Skipped
				mu.Unlock()
			}(rng[0], rng[1])
		}
		wg.Wait()

		if res.Parsed != validRows || callback != validRows {
			t.Errorf("workers=%d: parsed=%d callbacks=%d, want %d", workers, res.Parsed, callback, validRows)
		}
		if res.Skipped != 1 {
			t.Errorf("workers=%d: skipped=%d, want 1", workers, res.Skipped)
		}
	}
}

func TestProcessChunkMissingFile(t *testing.T) {
	if _, err := ProcessChunk("does-not-exist.csv", 0, 10, func(Row) {}); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestProcessChunkHeaderOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "header.csv")
	if err := os.WriteFile(path, []byte("campaign_id,date,impressions,clicks,spend,conversions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	res, err := ProcessChunk(path, 0, info.Size(), func(Row) {
		t.Fatal("onRow should not be called for a header-only file")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Parsed != 0 || res.Skipped != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
}
