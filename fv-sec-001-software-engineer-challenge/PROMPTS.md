# PROMPTS

Prompts used while building this solution with an AI coding assistant
(Claude Code). User prompts are pasted raw and unedited, in order, each
followed by a summary of the assistant's response.

---

## 1.

**Prompt:**

> Read the @README.md and analyze the requirements. Draft a plan for this task. The language of choose here is Go 1.24

**Response:**

Inspected the dataset and environment: `ad_data.csv` is 1.04 GB,
26,843,545 data rows, with only **50 distinct `campaign_id` values** — so the
aggregate result set is tiny and the real challenge is streaming 1 GB / ~27 M
rows fast in constant memory. Go 1.24.2 is installed.

Proposed a plan:

- **Memory:** stream the file, never hold rows; aggregate into a small map.
- **Speed:** parallel chunked reader — split the file into `GOMAXPROCS` byte
  ranges, each worker snaps to the next newline boundary, parses its range,
  merge partials.
- **Parsing:** manual byte-level comma split, not `encoding/csv`.
- **Decisions to document:** `float64` for spend; CTR = 0 when impressions = 0;
  CPA = null and excluded when conversions = 0; tie-break by `campaign_id`
  ascending; malformed rows skipped + counted.
- **Structure:** `cmd/aggregator` + `internal/parse` + `internal/aggregate` +
  `internal/output`, tests per package, stdlib-only, plus README / PROMPTS.md /
  Dockerfile / Makefile.
- Listed build order and flagged open questions (tie-breaking, parallel vs
  simple, output precision).

---

## 2.

**Prompt:**

> This `ad_data.csv` file is only an example of the input file. This program might be used on other input, with more diverse sets of `campaign_id`. In that case, the aggregation result set can grow a lot larger. Is the current approach still suitable?

**Response:**

Agreed the plan had a real weak spot. Key points:

- Aggregation memory is inherently `O(distinct keys)`, not `O(rows)` —
  streaming fixes rows, not cardinality.
- The specific bug: "each worker builds a partial map, merge at the end" means
  every worker holds *all* keys → memory is `O(keys × workers)`.
- Fixes proposed:
  1. **Partitioned aggregation** — hash `campaign_id` into shards so each key
     lives in exactly one aggregator; memory becomes `O(total keys)` flat.
  2. **Spill-to-disk fallback** — for cardinality beyond RAM, hash-partition to
     temp files and aggregate each partition (classic external group-by).
  3. **Bounded min-heap** of size N for ranking — `O(N)` space, never scales
     with cardinality.

Asked the user how far the scaling design should go. **User selected:
"Partitioned in-memory"** (hash-sharded aggregation + bounded heap; spill-to-
disk documented as the next step but not built).

---

## 3.

**Prompt:**

> Add 2 optional cli arguments:
> - The number of top results that is written into the result files (default 10)
> - The input file path (default `./ad_data.csv`)

**Response:**

Updated the CLI spec to three optional flags:

| Flag       | Default         | Description                                  |
|------------|-----------------|----------------------------------------------|
| `--input`  | `./ad_data.csv` | Input CSV path                               |
| `--output` | `./results`     | Output directory                             |
| `--top`    | `10`            | Number of top results per result file (≥ 1)  |

Knock-on changes: the bounded heap is sized by `--top` instead of a hardcoded
10; `--top` is validated to be ≥ 1. Noted that output filenames would stay
`top10_*.csv` (the names the submission requires) unless the user wanted them
to track N — and asked which they preferred.

---

## 4.

**Prompt:**

> Let the output files' names track N then process with this plan.

**Response:**

Confirmed output files would be named `top<N>_ctr.csv` / `top<N>_cpa.csv`
(default `--top 10` still yields the exact `top10_*.csv` names the submission
requires), then implemented the whole solution:

- Wrote all packages (`parse`, `aggregate`, `output`, `cmd/aggregator`) and
  per-package tests; stdlib-only.
- `gofmt` clean, `go vet` clean, all tests pass.
- Ran on the real 1 GB dataset: **~0.49 s**, **~20 MiB** peak RSS, 0 rows
  skipped, ~2 GB/s throughput.
- Cross-checked correctness against an independent `awk` aggregation — matched
  to the cent.
- Added `Dockerfile`, `Makefile`, `benchmark.log`, updated `.gitignore`, and
  rewrote `README.md` as the submission README (original challenge brief
  preserved in git history).
- Generated `results/top10_ctr.csv` and `results/top10_cpa.csv`.
