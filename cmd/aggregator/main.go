package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/duylong22197/aggregator/internal/aggregator"
	"github.com/duylong22197/aggregator/internal/csvreader"
	"github.com/duylong22197/aggregator/internal/ranking"
	"github.com/duylong22197/aggregator/internal/writer"
)

const (
	defaultOutputDir = "./results"
	topN             = 10
)

type config struct {
	input      string
	output     string
	workers    int
	cpuProfile string
	memProfile string
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	if cfg.cpuProfile != "" {
		f, err := os.Create(cfg.cpuProfile)
		if err != nil {
			return fmt.Errorf("create CPU profile: %w", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("start CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}

	start := time.Now()
	slog.Info("processing started", "input", cfg.input, "workers", cfg.workers, "output", cfg.output)

	ctx := context.Background()

	// Stage 1: stream CSV rows into jobs channel.
	jobs := make(chan []string, 10_000)
	reader := csvreader.New(csvreader.Config{FilePath: cfg.input})

	var readStats csvreader.ReadStats
	var readErr error
	readerDone := make(chan struct{})

	go func() {
		defer close(readerDone)
		readStats, readErr = reader.Read(ctx, jobs)
	}()

	// Stage 2: worker pool parses rows and aggregates into campaign stats map.
	agg := aggregator.New(aggregator.Config{Workers: cfg.workers})
	stats, err := agg.Run(ctx, jobs)

	// Wait for the reader goroutine to finish and check its error.
	<-readerDone
	if readErr != nil {
		return fmt.Errorf("read CSV: %w", readErr)
	}
	if err != nil {
		return fmt.Errorf("aggregate: %w", err)
	}

	elapsed := time.Since(start)
	slog.Info("processing complete",
		"rows_processed", readStats.RowsProcessed,
		"rows_skipped", readStats.RowsSkipped,
		"campaigns", len(stats),
		"elapsed", elapsed.Round(time.Millisecond),
	)

	// Stage 3: rank campaigns.
	topCTR := ranking.TopCTR(stats, topN)
	topCPA := ranking.TopCPA(stats, topN)

	// Stage 4: write output files.
	w := writer.New(writer.Config{OutputDir: cfg.output})
	if err := w.WriteCTR(topCTR); err != nil {
		return fmt.Errorf("write CTR file: %w", err)
	}
	if err := w.WriteCPA(topCPA); err != nil {
		return fmt.Errorf("write CPA file: %w", err)
	}

	slog.Info("output written",
		"ctr_file", w.CTRPath(),
		"cpa_file", w.CPAPath(),
		"total_elapsed", time.Since(start).Round(time.Millisecond),
	)

	if cfg.memProfile != "" {
		if err := writeMemProfile(cfg.memProfile); err != nil {
			return err
		}
	}

	return nil
}

func parseFlags() (config, error) {
	fs := flag.NewFlagSet("aggregator", flag.ContinueOnError)

	var cfg config
	fs.StringVar(&cfg.input, "input", "", "path to input CSV file (required)")
	fs.StringVar(&cfg.output, "output", defaultOutputDir, "output directory for result files")
	fs.IntVar(&cfg.workers, "workers", runtime.NumCPU(), "number of worker goroutines")
	fs.StringVar(&cfg.cpuProfile, "cpuprofile", "", "write CPU profile to file (optional)")
	fs.StringVar(&cfg.memProfile, "memprofile", "", "write memory profile to file (optional)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return config{}, err
	}

	return validate(cfg)
}

func validate(cfg config) (config, error) {
	if cfg.input == "" {
		return config{}, errors.New("--input is required")
	}

	if _, err := os.Stat(cfg.input); err != nil {
		if os.IsNotExist(err) {
			return config{}, fmt.Errorf("input file %q not found", cfg.input)
		}
		return config{}, fmt.Errorf("input file %q: %w", cfg.input, err)
	}

	if cfg.workers < 1 {
		return config{}, fmt.Errorf("--workers must be >= 1, got %d", cfg.workers)
	}

	return cfg, nil
}

func writeMemProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create memory profile: %w", err)
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("write memory profile: %w", err)
	}
	slog.Info("memory profile written", "path", path)
	return nil
}
