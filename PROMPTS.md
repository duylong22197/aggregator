# PROMPTS.md

## Prompt 1

You are a Senior Golang Engineer.

Build a production-quality Golang CLI application that processes a large CSV file (~1GB) containing advertising performance records and generates campaign-level analytics.

The goal is to demonstrate:

* Clean architecture
* Readable and maintainable code
* High performance
* Memory efficiency
* Concurrency where appropriate
* Error handling
* Testing
* Benchmarking
* Dockerization
* Production-ready engineering practices

# Dataset

Assume the CSV contains:

```csv
campaign_id,impressions,clicks,spend,conversions
CMP001,1000,50,100.50,5
CMP001,500,20,40.00,2
CMP002,2000,60,120.00,0
```

File size is approximately 1GB.

The solution MUST process the file without loading the entire dataset into memory.

Use a streaming approach.

---

# Functional Requirements

## Aggregate by campaign_id

For every campaign_id calculate:

* total_impressions
* total_clicks
* total_spend
* total_conversions

Derived metrics:

CTR = total_clicks / total_impressions

CPA = total_spend / total_conversions

Rules:

* If total_impressions = 0, CTR = 0
* If total_conversions = 0, CPA = null internally
* Campaigns with zero conversions must be excluded from CPA ranking

---

# Output Files

Generate:

## top10_ctr.csv

Top 10 campaigns with highest CTR.

Columns:

```text
campaign_id
total_impressions
total_clicks
total_spend
total_conversions
CTR
CPA
```

Sort:

```text
CTR DESC
```

Output example:

```csv
campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA
CMP042,125000,6250,12500.50,625,0.0500,20.00
CMP015,340000,15300,30600.25,1530,0.0450,20.00
```

---

## top10_cpa.csv

Top 10 campaigns with lowest CPA.

Exclude campaigns with zero conversions.

Sort:

```text
CPA ASC
```

Output example:

```csv
campaign_id,total_impressions,total_clicks,total_spend,total_conversions,CTR,CPA
CMP007,450000,13500,13500.00,1350,0.0300,10.00
CMP019,780000,23400,23400.00,2340,0.0300,10.00
```

---

# CLI Requirements

Application should support:

```bash
./aggregator \
  --input ad_data.csv \
  --output ./results \
  --workers 4
```

Required flags:

```text
--input
--output
--workers
```

Validate all inputs and return meaningful errors.

---

# Performance Requirements

The file size is approximately 1GB.

Design for:

## Streaming Processing

Use:

```go
bufio
encoding/csv
```

Read records row-by-row.

Do not load the full file into memory.

---

## Memory Efficiency

Only keep campaign aggregates in memory.

Avoid:

* loading all rows
* large intermediate slices
* unnecessary allocations

Target:

```text
Memory Complexity:
O(number_of_campaigns)
```

---

## Concurrency

Implement a worker-pool architecture.

Suggested flow:

```text
CSV Reader
     ↓
Jobs Channel
     ↓
Worker Pool
     ↓
Aggregation
     ↓
Ranking
     ↓
CSV Writer
```

Use:

```go
goroutines
channels
sync.WaitGroup
```

Avoid race conditions.

---

## Complexity Targets

```text
Time Complexity:
O(n)

Memory Complexity:
O(unique_campaigns)
```

---

# Ranking Implementation

Use efficient ranking.

Do not sort unnecessarily.

Use:

```go
container/heap
```

Maintain:

* Top 10 highest CTR
* Top 10 lowest CPA

Explain the approach in README.

---

# Error Handling

Handle gracefully:

* missing input file
* invalid CSV format
* malformed rows
* invalid numeric values
* missing columns
* output directory failures
* empty files

Never panic.

Return clear user-friendly error messages.

---

# Logging

Provide structured logs.

Log:

* file processing start
* rows processed
* malformed rows skipped
* processing completion
* execution time

Keep logs concise.

---

# Project Structure

Create:

```text
.
├── cmd/
│   └── aggregator/
│       └── main.go
├── internal/
│   ├── aggregator/
│   ├── csvreader/
│   ├── ranking/
│   ├── writer/
│   └── models/
├── tests/
├── benchmarks/
├── profiles/
├── Makefile
├── Dockerfile
├── README.md
├── go.mod
└── go.sum
```

---

# Testing Requirements

Create unit tests for:

## Aggregation

Verify:

* impressions aggregation
* clicks aggregation
* spend aggregation
* conversions aggregation

## Metrics

Verify:

* CTR calculation
* CPA calculation
* zero impression handling
* zero conversion handling

## Ranking

Verify:

* top CTR ordering
* top CPA ordering

## CSV Processing

Verify:

* malformed rows
* invalid numeric values
* empty file handling

Run with:

```bash
go test ./...
```

Aim for high coverage.

---

# Benchmarking Requirements

Create benchmark tests.

Provide:

```bash
go test -bench=. -benchmem ./...
```

Generate:

```text
benchmarks/benchmark.txt
```

Include benchmark instructions.

Create realistic benchmark tooling but DO NOT invent benchmark numbers.

Use actual measured values only.

---

# Memory Profiling

Add support for optional profiling.

Include examples using:

```go
runtime.ReadMemStats
```

and

```bash
go tool pprof
```

Generate:

```text
profiles/
```

Provide instructions in README.

---

# Docker Requirements

Create a production-ready multi-stage Dockerfile.

Requirements:

Stage 1:

* golang builder image

Stage 2:

* alpine or distroless image

Requirements:

* minimal image size
* non-root user
* statically compiled binary where possible

Build:

```bash
docker build -t campaign-aggregator .
```

Run:

```bash
docker run --rm \
  -v $(pwd)/data:/data \
  campaign-aggregator \
  --input /data/ad_data.csv \
  --output /data/results
```

---

# Makefile

Provide:

```bash
make build
make run
make test
make benchmark
make profile
make docker-build
make docker-run
```

---

# README Requirements

Create a professional README.md containing:

## Overview

Problem statement and solution summary.

## Architecture

Explain:

* reader
* worker pool
* aggregation
* ranking
* CSV writer

Include an architecture diagram using ASCII.

## Setup Instructions

Requirements:

* Go version
* Docker version

## Build Instructions

```bash
go mod tidy
go build -o aggregator ./cmd/aggregator
```

## Run Instructions

Local:

```bash
./aggregator \
  --input ad_data.csv \
  --output ./results \
  --workers 4
```

Docker:

```bash
docker build -t campaign-aggregator .
docker run ...
```

## Libraries Used

List all dependencies and justify them.

If only standard library is used, explicitly explain why.

## Benchmark Results

Create sections for:

* Dataset size
* Row count
* Campaign count
* Execution time
* Rows per second
* Peak memory usage

DO NOT fabricate numbers.

Use placeholders if benchmarks have not been executed.

Example:

```text
Dataset Size: <measured>
Rows: <measured>
Execution Time: <measured>
Peak Memory: <measured>
```

## Complexity Analysis

```text
Time Complexity: O(n)
Memory Complexity: O(unique_campaigns)
```

## Trade-offs and Design Decisions

Explain:

* streaming design
* worker pool design
* memory optimization
* malformed row strategy
* ranking strategy

## Testing

```bash
go test ./...
```

## Benchmarking

```bash
go test -bench=. -benchmem ./...
```

## Profiling

```bash
go tool pprof
```

---

# Code Quality Requirements

Follow idiomatic Go.

Requirements:

* small focused functions
* meaningful names
* clean package boundaries
* no dead code
* no commented-out code
* lint-friendly
* maintainable structure
* proper error wrapping
* clear comments only where needed

---

# Deliverables

Generate ALL of the following:

1. Complete source code
2. go.mod
3. Dockerfile
4. Makefile
5. Unit tests
6. Benchmark tests
7. README.md
8. Example output files
9. Benchmark placeholder files
10. Profiling instructions

Provide the full implementation, not partial snippets.

The final solution should be production-ready and capable of efficiently processing a ~1GB CSV file while maintaining low memory usage and predictable performance.

First, create the project structure, architecture plan, and README outline. Wait for my approval before implementing code.

## Prompt 2

Fix the readme file to correct the missing date column in the csv

## Prompt 3

So, what is the command to run if we have already builded

## Prompt 4

Could u fix this: note that in this project, the aggregator is a directory (the pre-existing one), so you need to reference the binary by the name you built it with. Since go build -o aggregator would conflict with that directory.

Also, I want to separate the build and the run command and show that in the README file so that it's easier for other devs to understand this code and use it

## Prompt 5

show me the source directory file

## Prompt 6

Why not use makefile command in readme?

## Prompt 7

Review this solution as if you were a senior engineer conducting a take-home assessment. Identify:
- bugs
- race conditions
- memory issues
- unnecessary complexity
- test gaps
- Go best-practice violations

Then fix all issues.

## Prompt 8

make gitignore for bin and coverage.out

## Prompt 9

Create a CLAUDE.md file explaining:
- architecture
- package responsibilities
- assumptions
- performance considerations
- benchmark procedure
- future improvements

## Prompt 10

Help me create PROMPTS.md file:
- Must be a file named exactly PROMPTS.md (no other format accepted)
- Paste my prompts as-is — raw, unedited, exactly as I typed them
- Do not clean up, polish, or rewrite my prompts

## Prompt 11

I don't think u should be separated by --- dividers. My first prompt contains that, so it could be hard to read. Try using another way to separate them. Maybe number it.
