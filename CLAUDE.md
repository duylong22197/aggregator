# Campaign Aggregator — CLAUDE.md

## Architecture

Three-stage pipeline. Each stage runs concurrently and communicates via buffered channels.

```
File
 │
 ▼
csvreader (1 goroutine)
 │  reads row-by-row, resolves columns from header, sends []string
 ▼
jobs channel (buffered, 10 000)
 │
 ▼
aggregator worker pool (N goroutines)
 │  parses and validates each []string into a typed Record
 ▼
results channel (buffered, 10 000)
 │
 ▼
aggregate goroutine (1 goroutine)
 │  owns map[campaign_id]*CampaignStats — no locks needed
 ▼
ranking (main goroutine)
 │  container/heap — top-10 CTR, top-10 CPA
 ▼
writer (main goroutine)
   writes top10_ctr.csv, top10_cpa.csv
```

### Goroutine lifecycle

1. Reader goroutine starts, streams rows into `jobs`, closes `jobs` on EOF or error.
2. N worker goroutines drain `jobs`, parse rows, send valid `Record` values to `results`.
3. A closer goroutine waits on `sync.WaitGroup` for all workers, then closes `results`.
4. The aggregate goroutine drains `results` until closed, returns the stats map.
5. Main calls ranking then writer sequentially.

Context cancellation is respected at two points: the `jobs <- extracted` send in the reader (select with `ctx.Done()`), and the `case <-ctx.Done()` arm in each worker's select. This ensures clean shutdown without goroutine leaks if the caller cancels.

---

## Package responsibilities

| Package | Role |
|---|---|
| `internal/models` | Shared value types only — `Record`, `CampaignStats`, `CampaignResult`. No logic. |
| `internal/csvreader` | File I/O. Opens file, validates header by name (not position), extracts the 5 required columns into a normalised `[]string`, streams to `jobs`. Skips malformed rows with a WARN log. |
| `internal/aggregator` | Worker pool. Each worker parses a `[]string` into a `Record` and validates all numeric fields (type + non-negative range). A single collector goroutine owns the stats map — no mutex required. |
| `internal/ranking` | Top-N selection via `container/heap`. `TopCTR` uses a min-heap of size 10; `TopCPA` uses a max-heap of size 10. Campaigns with zero conversions are excluded from CPA ranking. |
| `internal/writer` | Writes `top10_ctr.csv` and `top10_cpa.csv`. Creates output directory if missing. Calls `Flush()` and checks `Error()` explicitly — `encoding/csv` buffers writes silently. |
| `cmd/aggregator` | CLI wiring: flag parsing, validation, pipeline assembly, `log/slog` progress logging, optional `--cpuprofile` / `--memprofile` flags. |

---

## Assumptions

- **CSV schema**: the file must contain at minimum the five columns `campaign_id`, `impressions`, `clicks`, `spend`, `conversions`. Column order is irrelevant — positions are resolved from the header row. Extra columns (e.g. `date` in the real `ad_data.csv`) are ignored automatically.
- **Numeric fields**: `impressions`, `clicks`, `conversions` are integers; `spend` is a float64. All four must be non-negative. Rows that fail either condition are skipped with a WARN log and counted in `RowsSkipped`.
- **Empty campaign_id**: currently accepted. If rejected upstream, add a `row[0] == ""` guard in `parseRow`.
- **File size**: assumed to fit on local disk; the process reads it once sequentially. Random-access or multi-file inputs are not supported.
- **Worker count**: defaults to `runtime.NumCPU()`. The bottleneck is I/O, not CPU, so increasing workers beyond the core count yields diminishing returns.
- **Top-N**: hardcoded to 10 in `cmd/aggregator/main.go` (`topN = 10`). Changing it requires only modifying that constant.

---

## Performance considerations

- **Streaming**: `bufio.NewReaderSize` wraps the file with a 4 MB buffer, reducing syscall frequency. The CSV reader processes one row at a time — peak memory is O(unique campaigns), not O(rows).
- **Single aggregator goroutine**: the campaign stats map is owned by exactly one goroutine. This eliminates mutex contention on the hot aggregation path. Workers only do CPU work (parsing, validation); the map write is off the critical path.
- **Heap ranking**: O(m log 10) ≈ O(m) where m = unique campaigns. No full sort of all campaigns. The reverse-pop extraction from the heap already yields the correct order — no second sort pass is needed.
- **Channel buffer sizes**: `jobs` (10 000) and `results` (10 000) are sized to absorb burst throughput between stages. Tuning these can trade memory for fewer context switches.
- **`cr.ReuseRecord = false`**: deliberate. Rows are sent across goroutine boundaries via the channel; reusing the underlying slice would cause data races.

### Measured baseline (Apple M-series, 4 workers, ad_data.csv)

| Metric | Value |
|---|---|
| File size | ~995 MB |
| Rows | 26 843 544 |
| Unique campaigns | 50 |
| Execution time | ~11.4 s |
| Throughput | ~2 354 000 rows/s |

---

## Benchmark procedure

Run the micro-benchmarks:

```bash
make benchmark
# equivalent: go test -bench=. -benchmem -count=3 ./benchmarks/ | tee benchmarks/benchmark.txt
```

Available benchmarks in `benchmarks/bench_test.go`:

| Benchmark | Measures |
|---|---|
| `BenchmarkAggregation` | Worker pool + aggregation throughput, reports `rows/s` |
| `BenchmarkRankingTopCTR` | Heap ranking over 100k campaigns |
| `BenchmarkRankingTopCPA` | Heap ranking over 100k campaigns |
| `BenchmarkParseAndAggregate` | Combined parse + aggregate path, reports `MB/s` |

CPU and memory profiles:

```bash
make profile INPUT=ad_data.csv
# writes profiles/cpu.prof and profiles/mem.prof

go tool pprof -http=:8080 profiles/cpu.prof
go tool pprof -http=:8080 profiles/mem.prof
```

---

## Future improvements

- **Signal handling**: wire `SIGINT`/`SIGTERM` to a `context.WithCancel` so the pipeline shuts down cleanly on Ctrl-C instead of requiring SIGKILL.
- **Empty campaign_id validation**: add a guard in `parseRow` to reject rows where `campaign_id` is an empty string, preventing phantom entries in the stats map.
- **Configurable top-N**: expose `--top-n` as a CLI flag instead of the hardcoded constant in `main.go`.
- **Full CSV pipeline benchmark**: `generateCSV` was removed as dead code; a benchmark that exercises `csvreader` → `aggregator` end-to-end (streaming from an in-memory reader) would give a more realistic throughput number than the pre-parsed `[][]string` approach.
- **Output format options**: support JSON or Parquet output alongside CSV for downstream pipeline compatibility.
- **Parallel output writing**: `WriteCTR` and `WriteCPA` run sequentially; they could run in parallel goroutines since they write to independent files.
- **Structured error reporting**: collect all skipped-row errors into a summary report file rather than only logging them, making data-quality auditing easier in production.
