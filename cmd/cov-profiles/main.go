package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lndfuzz "github.com/lightninglabs/lnd-fuzz"
)

func main() {
	var (
		packagesFlag string
		verbose      bool
		dryRun       bool
		cacheDir     string
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] LND_DIR\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nFetches coverage data for each fuzz test, combines them, and produces a\n")
		fmt.Fprintf(os.Stderr, "coverage profile that can be analyzed.\n\n")
		fmt.Fprintf(os.Stderr, "After running the script, go to your lnd directory and run:\n")
		fmt.Fprintf(os.Stderr, "  go tool cover -html ../lnd-fuzz/coverage/profile\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&packagesFlag, "packages", "", "Comma-separated list of packages to test (default: all standard packages)")
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&dryRun, "dry-run", false, "analyze without writing coverage profile")
	flag.StringVar(&cacheDir, "cache-dir", "", "directory to use for fuzzing cache (default: temp directory)")
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	lndDir := flag.Arg(0)

	// Get base directory (where this command is run from)
	baseDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// If we're in a cmd subdirectory, go up to the root
	if filepath.Base(filepath.Dir(baseDir)) == "cmd" {
		baseDir = filepath.Dir(filepath.Dir(baseDir))
	}

	cfg := lndfuzz.CovProfilesConfig{
		LNDDir:   lndDir,
		BaseDir:  baseDir,
		CacheDir: cacheDir,
	}

	// Parse packages if provided
	if packagesFlag != "" {
		cfg.Packages = parsePackages(packagesFlag)
	} else {
		cfg.Packages = lndfuzz.DefaultPackages()
	}

	// Create coverage collector
	collector := lndfuzz.NewCoverageCollector(cfg)

	// Set up progress reporter
	reporter := &consoleReporter{verbose: verbose}
	collector.SetProgressReporter(reporter)

	// Print configuration
	fmt.Printf("Coverage Collection Configuration:\n")
	fmt.Printf("  LND directory: %s\n", lndDir)
	fmt.Printf("  Base directory: %s\n", baseDir)
	fmt.Printf("  Packages: %v\n", cfg.Packages)
	if cacheDir != "" {
		fmt.Printf("  Cache directory: %s\n", cacheDir)
	}
	if dryRun {
		fmt.Printf("  Mode: dry-run (analyze only)\n")
	}
	fmt.Println()

	// Collect coverage
	result, err := collector.Collect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error collecting coverage: %v\n", err)
		os.Exit(1)
	}

	// Print results
	fmt.Printf("\nCoverage Collection Results:\n")
	fmt.Printf("  Total targets: %d\n", len(result.Targets))

	successful := 0
	failed := 0
	totalInputs := 0
	for _, target := range result.Targets {
		if target.Error != nil {
			failed++
			if verbose {
				fmt.Printf("  ✗ %s/%s: %v\n", target.Package, target.Target, target.Error)
			}
		} else {
			successful++
			totalInputs += target.NumInputs
			if verbose {
				fmt.Printf("  ✓ %s/%s: %d inputs\n", target.Package, target.Target, target.NumInputs)
			}
		}
	}

	fmt.Printf("  Successful: %d\n", successful)
	fmt.Printf("  Failed: %d\n", failed)
	fmt.Printf("  Total inputs: %d\n", totalInputs)
	fmt.Printf("  Duration: %v\n", result.EndTime.Sub(result.StartTime))

	// Write to disk if not dry-run
	if !dryRun {
		if err := collector.Write(result); err != nil {
			fmt.Fprintf(os.Stderr, "\nError writing coverage profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nCoverage profile written to: %s\n", result.ProfilePath)
		fmt.Println("\nView coverage in HTML with:")
		fmt.Printf("  cd %s && go tool cover -html ../lnd-fuzz/coverage/profile\n", lndDir)
	} else {
		fmt.Println("\nDry-run complete. No files were written.")
	}
}

func parsePackages(packagesStr string) []string {
	var packages []string
	for _, pkg := range strings.Split(packagesStr, ",") {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" {
			packages = append(packages, pkg)
		}
	}
	return packages
}

// consoleReporter implements lndfuzz.ProgressReporter for console output.
type consoleReporter struct {
	verbose bool
}

func (r *consoleReporter) ReportProgress(current, total int, message string) {
	if r.verbose {
		fmt.Printf("[%d/%d] %s\n", current, total, message)
	}
}

func (r *consoleReporter) ReportInfo(message string) {
	if r.verbose {
		fmt.Printf("INFO: %s\n", message)
	}
}

func (r *consoleReporter) ReportWarning(message string) {
	fmt.Printf("WARNING: %s\n", message)
}

func (r *consoleReporter) ReportError(message string) {
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", message)
}
