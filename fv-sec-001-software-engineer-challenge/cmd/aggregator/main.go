// Command aggregator reads a large ad-performance CSV and writes the top-N
// campaigns by CTR and by CPA as CSV files.
//
// Usage:
//
//	aggregator [--input ./ad_data.csv] [--output ./results] [--top 10]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"adperf/internal/aggregate"
	"adperf/internal/output"
	"adperf/internal/parse"
)

// shardsPerWorker sizes the aggregator's shard count relative to the worker
// count, trading a little memory for lower lock contention.
const shardsPerWorker = 16

func main() {
	log.SetFlags(0)

	input := flag.String("input", "./ad_data.csv", "path to the input CSV file")
	outputDir := flag.String("output", "./results", "directory for the result CSV files")
	top := flag.Int("top", 10, "number of campaigns to write to each result file")
	flag.Parse()

	if *top < 1 {
		log.Fatalf("aggregator: --top must be at least 1, got %d", *top)
	}
	if err := run(*input, *outputDir, *top); err != nil {
		log.Fatalf("aggregator: %v", err)
	}
}

func run(input, outputDir string, top int) error {
	start := time.Now()

	info, err := os.Stat(input)
	if err != nil {
		return fmt.Errorf("cannot read input file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("input %q is a directory, not a file", input)
	}

	workers := runtime.NumCPU()
	ranges := parse.SplitRanges(info.Size(), workers)
	agg := aggregate.NewShardedAggregator(len(ranges) * shardsPerWorker)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		total parse.ChunkResult
		errs  []error
	)
	for _, rng := range ranges {
		wg.Add(1)
		go func(start, end int64) {
			defer wg.Done()
			combiner := agg.NewCombiner()
			res, err := parse.ProcessChunk(input, start, end, combiner.Add)
			combiner.Flush()

			mu.Lock()
			total.Parsed += res.Parsed
			total.Skipped += res.Skipped
			if err != nil {
				errs = append(errs, err)
			}
			mu.Unlock()
		}(rng[0], rng[1])
	}
	wg.Wait()
	if len(errs) > 0 {
		return fmt.Errorf("processing input: %w", errs[0])
	}

	byCTR := agg.TopByCTR(top)
	byCPA := agg.TopByCPA(top)
	if err := output.WriteResults(outputDir, top, byCTR, byCPA); err != nil {
		return err
	}

	reportSummary(input, outputDir, top, workers, total, time.Since(start))
	return nil
}

// reportSummary prints run statistics to stderr. The Go heap figures are a
// rough guide; use `/usr/bin/time -v` for true peak resident memory.
func reportSummary(input, outputDir string, top, workers int, total parse.ChunkResult, elapsed time.Duration) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	const mib = 1 << 20

	log.Printf("input:          %s", input)
	log.Printf("rows parsed:    %d", total.Parsed)
	log.Printf("rows skipped:   %d", total.Skipped)
	log.Printf("workers:        %d", workers)
	log.Printf("elapsed:        %s", elapsed.Round(time.Millisecond))
	log.Printf("go heap (live): %.1f MiB", float64(mem.HeapAlloc)/mib)
	log.Printf("go mem (sys):   %.1f MiB", float64(mem.Sys)/mib)
	log.Printf("results:        %s/top%d_ctr.csv, %s/top%d_cpa.csv", outputDir, top, outputDir, top)
}
