# Campaign Aggregator

A production-quality Go CLI that processes a ~1 GB advertising CSV file, aggregates performance metrics per campaign, and writes top-10 ranked output files — all with O(unique_campaigns) memory and O(n) time complexity.

---

## Overview

**Problem**: Given a large CSV (≈1 GB) with rows like:

```
campaign_id,date,impressions,clicks,spend,conversions
CMP001,2025-04-18,1000,50,100.50,5
```

The reader resolves column positions from the header row, so extra columns such as `date` are automatically ignored — no code changes required for schema variations.

compute per-campaign totals and derived metrics (CTR, CPA), then output:

- `top10_ctr.csv` — 10 campaigns with the highest click-through rate
- `top10_cpa.csv` — 10 campaigns with the lowest cost-per-acquisition (zero-conversion campaigns excluded)

**Constraints**: the file must never be fully loaded into memory; only per-campaign running totals are kept.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          Pipeline Architecture                           │
│                                                                          │
│  ┌──────────────┐    ┌──────────────────┐    ┌───────────────────────┐   │
│  │  CSV Reader  │───▶│  jobs channel    │───▶│  Worker Pool (N)      │   │
│  │  1 goroutine │    │  buffered []str  │    │  parse + validate     │   │
│  │  bufio 4MB   │    │                  │    │  []string → Record    │   │
│  └──────────────┘    └──────────────────┘    └────────────┬──────────┘   │
│                                                           │              │
│                                               ┌───────────▼───────────┐  │
│                                               │  results channel      │  │
│                                               │  buffered Record      │  │
│                                               └───────────┬───────────┘  │
│                                                           │              │
│                                               ┌───────────▼───────────┐  │
│                                               │  Aggregator goroutine │  │
│                                               │  map[id]*Stats        │  │
│                                               └───────────┬───────────┘  │
│                                                           │              │
│                              ┌────────────────────────────┘              │
│                              │                                           │
│                 ┌────────────▼─────────────┐                             │
│                 │  container/heap ranking  │                             │
│                 │  top-10 CTR  top-10 CPA  │                             │
│                 └────────────┬─────────────┘                             │
│                              │                                           │
│                 ┌────────────▼─────────────┐                             │
│                 │  CSV Writer              │                             │
│                 │  top10_ctr.csv           │                             │
│                 │  top10_cpa.csv           │                             │
│                 └──────────────────────────┘                             │
└──────────────────────────────────────────────────────────────────────────┘
```

### Package Roles

| Package | Responsibility |
|---|---|
| `internal/csvreader` | Streams file row-by-row using `bufio` + `encoding/csv`. Validates the header dynamically so extra columns (e.g. `date`) are handled transparently. Malformed rows are skipped with a warning. |
| `internal/aggregator` | N-worker pool that parses raw `[]string` rows into typed `Record` values. A single collector goroutine owns the `map[string]*CampaignStats` — no locks needed. |
| `internal/ranking` | Two `container/heap` implementations. A min-heap tracks top-10 CTR; a max-heap tracks the 10 campaigns with the lowest CPA. Both run in O(m log 10) ≈ O(m). |
| `internal/writer` | Writes output CSV files with explicit `Flush()` + `Error()` checking. Creates the output directory if it does not exist. |
| `internal/models` | Shared value types: `Record`, `CampaignStats`, `CampaignResult`. |
| `cmd/aggregator` | CLI entry point: flag parsing, validation, pipeline wiring, structured logging, optional pprof. |

### Goroutine Lifecycle

1. Reader goroutine opens the file, streams rows into `jobs`, closes `jobs` on EOF.
2. N worker goroutines drain `jobs`, parse rows, send valid `Record` values to `results`.
3. A closer goroutine waits for all workers (`sync.WaitGroup`) then closes `results`.
4. One aggregator goroutine drains `results` and upserts into the stats map; returns when `results` is closed.
5. Main runs ranking and writing sequentially.

---

## Setup

**Requirements**:

- Go 1.22+
- Docker 20.10+ (for containerised builds)

```bash
# Verify Go version
go version
```

---

## Build & Run

### Local

**Step 1 — Build** (compile the binary once):

```bash
make build
```

The binary is written to `bin/aggregator`.

**Step 2 — Run** (after the binary exists, re-run as many times as needed without rebuilding):

```bash
make run
# defaults: INPUT=ad_data.csv  OUTPUT=./results  WORKERS=4
```

> The default `INPUT=ad_data.csv` expects the file to be in the project root. If your CSV is elsewhere, override the path:

```bash
make run INPUT=/path/to/ad_data.csv OUTPUT=./results WORKERS=4
```

Only rebuild (`make build`) when you change the source code.

### All Makefile commands

```bash
make build            # compile binary → bin/aggregator
make run              # build + run (INPUT=ad_data.csv OUTPUT=./results WORKERS=4)
make test             # run all tests with race detector
make benchmark        # benchmark suite → benchmarks/benchmark.txt
make profile          # run with pprof (INPUT=... required)
make lint             # go vet + golangci-lint (if installed)
make clean            # remove bin/ and profiles
```

### Docker

```bash
# Build image (~5 MB distroless)
docker build -t campaign-aggregator .

# Run (mount a data directory)
docker run --rm \
  -v $(pwd)/data:/data \
  campaign-aggregator \
  --input /data/ad_data.csv \
  --output /data/results \
  --workers 4
```

---

## Libraries Used

**Standard library only** — no external dependencies.

| Package | Purpose |
|---|---|
| `encoding/csv` | Streaming CSV parsing |
| `bufio` | 4 MB read buffer to reduce syscall overhead |
| `container/heap` | O(log k) top-N ranking |
| `sync` | `WaitGroup` for worker lifecycle |
| `log/slog` | Structured logging (stable since Go 1.21) |
| `strconv` | Numeric field parsing |
| `flag` | CLI flag parsing |
| `runtime/pprof` | Optional CPU/memory profiling |

Using only the standard library eliminates supply-chain risk, reduces binary size, and removes `go mod tidy` friction for contributors.

---

## Benchmark Results

Run the benchmarks yourself:

```bash
go test -bench=. -benchmem -count=3 ./benchmarks/ | tee benchmarks/benchmark.txt
```

| Metric | Value |
|---|---|
| Dataset Size | ~995 MB |
| Row Count | 26,843,544 |
| Campaign Count | 50 |
| Execution Time | ~11.4 s (4 workers) |
| Rows / second | ~2,354,000 |
| Peak Memory | O(unique_campaigns) — negligible above runtime baseline |

*Measured against `ad_data.csv` on Apple M-series hardware. Run the benchmark commands above to reproduce.*

---

## Complexity Analysis

| Dimension | Complexity | Notes |
|---|---|---|
| Time | O(n) | Each row is read once, parsed once, aggregated once |
| Memory | O(unique_campaigns) | Only per-campaign running totals kept in memory |
| Ranking | O(m log k) | m = unique campaigns, k = 10; effectively O(m) |

---

## Trade-offs and Design Decisions

### Streaming over batch loading
The file is read row-by-row via `encoding/csv` wrapped in a 4 MB `bufio` reader. This keeps memory usage flat regardless of file size — only the campaign stats map grows, bounded by the number of unique campaigns.

### Pipeline concurrency (reader → workers → single aggregator)
Workers parse raw string values into typed structs in parallel, which is the CPU-intensive step. A *single* goroutine owns the aggregation map, so no mutex is needed and there is no lock contention. The alternative (mutex-protected map shared by all workers) would serialise all map writes and add overhead.

### `container/heap` for top-N ranking
Sorting all campaigns after aggregation would require O(m log m) time and O(m) extra memory. By maintaining a bounded heap of size 10, we achieve O(m log 10) ≈ O(m) time with O(1) extra memory. Two separate heaps are used:
- **CTR min-heap**: root holds the lowest CTR in the current top-10; evicts the worst when a better campaign arrives.
- **CPA max-heap**: root holds the highest CPA in the current top-10 cheapest set; evicts the most expensive when a cheaper campaign arrives.

### Malformed row strategy: skip and log
Rows with invalid numeric fields are counted and logged at WARN level. The pipeline never panics on bad data. This is appropriate for ETL over large files where a handful of corrupt rows should not abort a multi-minute job.

### `*float64` for CPA
Using a pointer allows `nil` to represent "no conversions" in a type-safe way. This propagates through the ranking (where zero-conversion campaigns are filtered out before the heap) to the writer (where `nil` formats as `"null"`), with no string parsing or sentinel values needed.

### Header-based column discovery
The CSV reader resolves column indices from the header row rather than using fixed positions. This means the pipeline handles files with extra columns (like the real `ad_data.csv` which includes a `date` column) without any code change.

---

## Testing

```bash
# All tests with race detector
go test -race -count=1 ./...

# With coverage report
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

Test coverage includes:

- Unit tests for each package (`csvreader`, `aggregator`, `ranking`, `writer`)
- Table-driven edge cases: zero impressions, zero conversions, malformed rows, extra columns
- End-to-end integration test in `tests/integration_test.go`

---

## Benchmarking

```bash
go test -bench=. -benchmem -count=3 ./benchmarks/
```

Benchmarks cover:

| Benchmark | What it measures |
|---|---|
| `BenchmarkAggregation` | Worker pool throughput (rows/s) across 100k rows |
| `BenchmarkRankingTopCTR` | Top-10 CTR heap over 100k campaigns |
| `BenchmarkRankingTopCPA` | Top-10 CPA heap over 100k campaigns |
| `BenchmarkParseAndAggregate` | Combined parse + aggregate (MB/s) |

---

## Profiling

### CPU profile

```bash
make profile INPUT=ad_data.csv
go tool pprof -http=:8080 profiles/cpu.prof
```

### Memory profile

```bash
make profile INPUT=ad_data.csv
go tool pprof -http=:8080 profiles/mem.prof
```

### Runtime memory stats (manual)

To inspect memory usage at a point in time, add to your code:

```go
var ms runtime.MemStats
runtime.ReadMemStats(&ms)
fmt.Printf("Alloc=%v MiB HeapInuse=%v MiB\n",
    ms.Alloc/1024/1024, ms.HeapInuse/1024/1024)
```
