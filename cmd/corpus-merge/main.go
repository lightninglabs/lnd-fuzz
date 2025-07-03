package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	lndfuzz "github.com/lightninglabs/lnd-fuzz"
)

// cliProgressReporter implements ProgressReporter for CLI output.
type cliProgressReporter struct {
	verbose bool
}

func (r *cliProgressReporter) ReportProgress(current, total int, message string) {
	if r.verbose {
		fmt.Printf("\r\033[2K[%d/%d] %s", current, total, message)
	} else {
		fmt.Printf("\r\033[2KMeasuring coverage for input %d/%d", current, total)
	}
}

func (r *cliProgressReporter) ReportInfo(message string) {
	// Clear the progress line and print info
	fmt.Printf("\r\033[2K%s\n", message)
}

func (r *cliProgressReporter) ReportWarning(message string) {
	fmt.Printf("\r\033[2KWarning: %s\n", message)
}

func (r *cliProgressReporter) ReportError(message string) {
	fmt.Printf("\r\033[2KError: %s\n", message)
}

func main() {
	var (
		verbose   bool
		dryRun    bool
		cacheDir  string
	)

	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&dryRun, "dry-run", false, "analyze without copying files")
	flag.StringVar(&cacheDir, "cache-dir", "", "custom cache directory")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] DEST_CORPUS SRC_CORPUS PACKAGE_DIR FUZZ_TARGET_NAME\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nMerges new fuzzing inputs into an existing corpus, only keeping inputs\n")
		fmt.Fprintf(os.Stderr, "that increase coverage as measured by Go's native fuzzing engine.\n")
		fmt.Fprintf(os.Stderr, "Prefers smaller inputs over larger ones.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s lnwire/testdata/fuzz/FuzzPong \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "      $(go env GOCACHE)/fuzz/github.com/lightningnetwork/lnd/lnwire/FuzzPong \\\n")
		fmt.Fprintf(os.Stderr, "      ../lnd/lnwire FuzzPong\n")
	}

	flag.Parse()

	if flag.NArg() != 4 {
		flag.Usage()
		os.Exit(1)
	}

	cfg := lndfuzz.CorpusMergeConfig{
		DestDir:    flag.Arg(0),
		SrcDir:     flag.Arg(1),
		PackageDir: flag.Arg(2),
		FuzzTarget: flag.Arg(3),
		CacheDir:   cacheDir,
	}

	// Create merger
	merger := lndfuzz.NewCorpusMerger(cfg)
	merger.SetProgressReporter(&cliProgressReporter{verbose: verbose})

	// Analyze
	fmt.Println("Analyzing corpus...")
	result, err := merger.Analyze()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	fmt.Printf("\nAnalysis complete (took %s)\n", result.EndTime.Sub(result.StartTime).Round(time.Millisecond))
	fmt.Printf("Baseline coverage: %d\n", result.BaselineCoverage)
	fmt.Printf("Final coverage: %d\n", result.FinalCoverage)
	fmt.Printf("Inputs analyzed: %d\n", result.InputsAnalyzed)
	fmt.Printf("Inputs to add: %d\n", result.InputsAdded)

	if verbose && result.InputsAdded > 0 {
		fmt.Println("\nInputs that increase coverage:")
		for _, input := range result.Inputs {
			if input.Added {
				fmt.Printf("  %s (size: %d, coverage increase: +%d)\n", 
					input.Name, input.Size, input.CoverageIncrease)
			}
		}
	}

	// Apply changes if not dry run
	if !dryRun && result.InputsAdded > 0 {
		fmt.Printf("\nCopying %d new inputs to destination...\n", result.InputsAdded)
		if err := merger.Apply(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying changes: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Done!")
	} else if dryRun && result.InputsAdded > 0 {
		fmt.Println("\n(Dry run - no files were copied)")
	}
}