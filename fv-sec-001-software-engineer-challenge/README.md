# Ad Performance Aggregator

A console application that processes a large advertising-performance CSV
(~1 GB, ~27 M rows) and produces aggregated analytics: per-campaign totals and
the top-N campaigns ranked by CTR and by CPA.

> The original challenge brief (FV-SEC001) is preserved in the git history of
> this repository.

## Requirements

- Go **1.24** or newer
- No third-party dependencies — standard library only

## Setup

```bash
# 1. Unzip the dataset (kept out of git via .gitignore)
unzip ad_data.csv.zip

# 2. Build
go build -o aggregator ./cmd/aggregator
#   or: make build
```

## Usage

```bash
./aggregator [--input ./ad_data.csv] [--output ./results] [--top 10]
```

| Flag       | Default          | Description                                            |
|------------|------------------|--------------------------------------------------------|
| `--input`  | `./ad_data.csv`  | Path to the input CSV file                             |
| `--output` | `./results`      | Directory for the result files (created if missing)    |
| `--top`    | `10`             | Number of campaigns written to each result file (≥ 1)  |

Example:

```bash
./aggregator --input ./ad_data.csv --output ./results --top 10
```

The program prints a short run summary (rows parsed/skipped, worker count,
elapsed time, memory) to stderr.

## Output

Two CSV files are written to the output directory, named after `--top`:

- `top<N>_ctr.csv` — the N campaigns with the **highest CTR**
- `top<N>_cpa.csv` — the N campaigns with the **lowest CPA** (campaigns with
  zero conversions are excluded)

With the default `--top 10` these are exactly `top10_ctr.csv` and
`top10_cpa.csv`. Both files share the column layout:

```
campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA
```

Sample output (`results/top10_ctr.csv`) is included in this repository.

## Per-campaign metrics

For each `campaign_id` the program computes `total_impressions`,
`total_clicks`, `total_spend`, `total_conversions`, and:

- **CTR** = `total_clicks / total_impressions` (0 when there are no impressions)
- **CPA** = `total_spend / total_conversions` (null — written as an empty
  field — when there are no conversions)

## How it works

The design keeps memory bounded by the number of *distinct campaigns*, never
by the size of the input, while using all CPU cores for parsing.

1. **Parallel chunked reading.** The input file is split into one contiguous
   byte range per CPU core. Each worker seeks to its range and realigns to the
   next line boundary, so a row split across a boundary is processed by exactly
   one worker — no row is dropped or double-counted. Workers stream their range
   through a 1 MiB buffer; the file is never loaded into memory.

2. **Fast row parsing.** Rows are parsed directly from the read buffer by
   scanning for commas — no `encoding/csv` overhead. The `campaign_id` is
   returned as a slice that aliases the buffer, so no string is allocated per
   row. Malformed rows (wrong field count, non-numeric/negative values, empty
   campaign id) are skipped and counted rather than aborting the run.

3. **Partitioned aggregation.** Totals live in a `ShardedAggregator` whose
   keyspace is partitioned across many independently locked shards. Each
   `campaign_id` belongs to exactly one shard, so total memory is
   `O(distinct campaigns)` regardless of the worker count — there is no
   per-worker copy of the full keyspace.

4. **Per-worker combiner.** Each worker first folds rows into a small local map
   and flushes it into the shared shards periodically (and once at the end).
   This cuts lock traffic to roughly one acquisition per *key per flush*
   instead of one per row, and bounds each worker's local memory.

5. **Bounded-heap ranking.** The top-N lists are built with a size-N min-heap
   in `O(distinct campaigns · log N)` time and `O(N)` space — the ranking step
   never materialises a full sort of the keyspace.

### Scaling note

This solution is built for input with a *large and diverse* set of
`campaign_id` values, not just the sample dataset. Steps 3–5 ensure memory and
ranking scale with cardinality, not file size, so millions of distinct
campaigns are handled comfortably in flat, low memory. If cardinality ever
exceeded available RAM, the natural next step would be a spill-to-disk pass
(hash-partition rows to temp files, aggregate each partition independently);
the partitioned architecture above is already the right shape for that.

## Design decisions

- **Tie-breaking.** The brief does not specify it. Ties in CTR/CPA are broken
  by `campaign_id` ascending, so output is fully deterministic.
- **CTR with zero impressions** is `0` — a campaign that was never shown cannot
  be ranked by click rate.
- **CPA with zero conversions** is undefined: such campaigns are excluded from
  the CPA list and show an empty CPA field in the CTR file.
- **Malformed rows are skipped, not fatal**, and reported in the run summary.
  Missing/unreadable input files are fatal with a clear message.
- **`spend` is accumulated as `float64`.** Sums of decimal values carry the
  usual floating-point imprecision; this matched the reference output to the
  cent on the sample dataset. Accumulating integer cents would be the
  alternative if exact decimal arithmetic were required.
- **Output precision:** CTR to 4 decimal places, CPA and spend to 2 — matching
  the formats in the challenge brief.
- **Worker count** defaults to `runtime.NumCPU()`.

## Performance

Measured on the provided 1 GB dataset (`ad_data.csv`, 26,843,544 data rows),
16 CPU cores, Go 1.24.2 — see [`benchmark.log`](./benchmark.log).

| Metric                 | Result                          |
|------------------------|---------------------------------|
| Processing time (1 GB) | **~0.49 s** (5-run avg ~0.49 s) |
| Peak resident memory   | **~20 MiB** (`VmHWM`)           |
| Rows parsed / skipped  | 26,843,544 / 0                  |

Throughput is roughly **2 GB/s** / ~55 M rows/s. Memory stays flat because the
file is streamed and only ~50 distinct campaigns are retained; on
high-cardinality input memory grows only with the campaign count.

## Testing

```bash
go test ./...
#   or: make test
```

Tests cover row parsing and edge cases, chunk-boundary correctness (every row
counted exactly once across many split counts), aggregation and concurrent
shard access, the combiner's periodic-flush path, ranking with tie-breaks and
zero-conversion exclusion, and CSV output formatting (including null CPA).

Correctness on the real dataset was additionally cross-checked against an
independent `awk` aggregation.

## Docker

```bash
docker build -t adperf .
docker run --rm -v "$PWD:/data" -w /data adperf \
  --input /data/ad_data.csv --output /data/results --top 10
```

## Project layout

```
cmd/aggregator/      CLI entry point and orchestration
internal/parse/      file splitting, chunk reading, row parsing
internal/aggregate/  sharded aggregator, per-worker combiner, top-N ranking
internal/output/     CSV result writers
benchmark.log        timing and memory measurements
```
