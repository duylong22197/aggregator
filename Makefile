BINARY      := bin/aggregator
CMD_PKG     := ./cmd/aggregator
MODULE      := github.com/duylong22197/aggregator
IMAGE       := campaign-aggregator

INPUT       ?= ad_data.csv
OUTPUT      ?= ./results
WORKERS     ?= 4

LDFLAGS     := -ldflags="-w -s"
GOFLAGS     :=

.PHONY: build run test benchmark profile docker-build docker-run lint clean

## build: compile the aggregator binary to bin/aggregator
build:
	mkdir -p bin
	go build $(LDFLAGS) -o $(BINARY) $(CMD_PKG)

## run: build and execute against INPUT file
run: build
	./$(BINARY) --input $(INPUT) --output $(OUTPUT) --workers $(WORKERS)

## test: run all unit and integration tests
test:
	go test -race -count=1 ./...

## benchmark: run all benchmarks and tee output to benchmarks/benchmark.txt
benchmark:
	go test -bench=. -benchmem -count=3 ./benchmarks/ | tee benchmarks/benchmark.txt

## profile: run with CPU and memory profiling enabled (requires INPUT to be set)
profile: build
	mkdir -p profiles
	./$(BINARY) \
	  --input $(INPUT) \
	  --output $(OUTPUT) \
	  --workers $(WORKERS) \
	  --cpuprofile profiles/cpu.prof \
	  --memprofile profiles/mem.prof
	@echo ""
	@echo "Profiles written to profiles/. Inspect with:"
	@echo "  go tool pprof -http=:8080 profiles/cpu.prof"
	@echo "  go tool pprof -http=:8080 profiles/mem.prof"

## docker-build: build the Docker image
docker-build:
	docker build -t $(IMAGE) .

## docker-run: run the aggregator inside Docker (mounts ./data as /data)
docker-run:
	docker run --rm \
	  -v "$$(pwd)/data:/data" \
	  $(IMAGE) \
	  --input /data/ad_data.csv \
	  --output /data/results \
	  --workers $(WORKERS)

## lint: run go vet (and golangci-lint if available)
lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || true

## clean: remove build artefacts
clean:
	rm -rf bin/
	rm -rf profiles/*.prof

## help: print this help message
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
