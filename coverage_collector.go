package lndfuzz

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CoverageCollector handles collection of coverage profiles from multiple fuzz targets.
type CoverageCollector struct {
	Config   CovProfilesConfig
	fs       FileSystem
	runner   CommandRunner
	progress ProgressReporter
}

// NewCoverageCollector creates a new CoverageCollector with the given configuration.
func NewCoverageCollector(cfg CovProfilesConfig) *CoverageCollector {
	// Set defaults
	if cfg.FS == nil {
		cfg.FS = OSFileSystem{}
	}
	if cfg.CmdRunner == nil {
		cfg.CmdRunner = &OSCommandRunner{}
	}
	if len(cfg.Packages) == 0 {
		cfg.Packages = DefaultPackages()
	}
	
	return &CoverageCollector{
		Config:   cfg,
		fs:       cfg.FS,
		runner:   cfg.CmdRunner,
		progress: NullProgressReporter{},
	}
}

// SetProgressReporter sets the progress reporter for this collector.
func (c *CoverageCollector) SetProgressReporter(reporter ProgressReporter) {
	if reporter != nil {
		c.progress = reporter
	} else {
		c.progress = NullProgressReporter{}
	}
}

// Collect collects coverage profiles from all fuzz targets.
// It returns the coverage data without writing to disk.
func (c *CoverageCollector) Collect() (*CoverageResult, error) {
	result := &CoverageResult{
		StartTime: time.Now(),
		Targets:   []CoverageTarget{},
	}
	
	// Create cache directory if not provided
	cacheDir := c.Config.CacheDir
	if cacheDir == "" {
		tmpDir, err := c.fs.TempDir("", "fuzz-coverage-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer c.fs.RemoveAll(tmpDir)
		cacheDir = tmpDir
	}
	
	var coverageDirs []string
	totalTargets := 0
	
	// Count total targets for progress reporting
	for _, pkg := range c.Config.Packages {
		fuzzDir := filepath.Join(c.Config.BaseDir, pkg, "testdata", "fuzz")
		if entries, err := c.fs.ReadDir(fuzzDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					totalTargets++
				}
			}
		}
	}
	
	currentTarget := 0
	
	// Collect coverage profiles for each package
	for _, pkg := range c.Config.Packages {
		fuzzDir := filepath.Join(c.Config.BaseDir, pkg, "testdata", "fuzz")
		
		// Check if fuzz directory exists
		if _, err := c.fs.Stat(fuzzDir); os.IsNotExist(err) {
			c.progress.ReportInfo(fmt.Sprintf("Skipping %s: no fuzz directory found", pkg))
			continue
		}
		
		// List all fuzz targets
		entries, err := c.fs.ReadDir(fuzzDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read fuzz dir %s: %w", fuzzDir, err)
		}
		
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			
			currentTarget++
			fuzzTarget := entry.Name()
			
			target := CoverageTarget{
				Package: pkg,
				Target:  fuzzTarget,
			}
			
			c.progress.ReportProgress(currentTarget, totalTargets,
				fmt.Sprintf("Collecting coverage for %s/%s", pkg, fuzzTarget))
			
			// Process this target
			coverageDir, numInputs, err := c.collectTargetCoverage(
				pkg, fuzzTarget, cacheDir,
			)
			if err != nil {
				target.Error = err
				c.progress.ReportError(fmt.Sprintf("Failed to collect coverage for %s/%s: %v",
					pkg, fuzzTarget, err))
			} else {
				target.NumInputs = numInputs
				target.CoverageDir = coverageDir
				coverageDirs = append(coverageDirs, coverageDir)
			}
			
			result.Targets = append(result.Targets, target)
		}
	}
	
	if len(coverageDirs) == 0 {
		return nil, fmt.Errorf("no coverage data collected")
	}
	
	// Combine coverage profiles
	c.progress.ReportInfo("Combining coverage profiles...")
	
	profileData, err := c.combineCoverageProfiles(coverageDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to combine coverage profiles: %w", err)
	}
	
	result.CombinedProfile = profileData
	result.ProfilePath = filepath.Join(c.Config.BaseDir, "coverage", "profile")
	result.EndTime = time.Now()
	
	return result, nil
}

// Write writes the coverage result to disk.
func (c *CoverageCollector) Write(result *CoverageResult) error {
	if result == nil {
		return fmt.Errorf("no coverage result provided")
	}
	
	if len(result.CombinedProfile) == 0 {
		return fmt.Errorf("no coverage data to write")
	}
	
	// Create coverage directory
	coverageDir := filepath.Dir(result.ProfilePath)
	if err := c.fs.MkdirAll(coverageDir, 0755); err != nil {
		return fmt.Errorf("failed to create coverage directory: %w", err)
	}
	
	// Write profile
	f, err := c.fs.Create(result.ProfilePath)
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer f.Close()
	
	if _, err := f.Write(result.CombinedProfile); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
	}
	
	return nil
}

// collectTargetCoverage collects coverage for a single fuzz target.
func (c *CoverageCollector) collectTargetCoverage(pkg, fuzzTarget, cacheDir string) (string, int, error) {
	fuzzTargetDir := filepath.Join(c.Config.BaseDir, pkg, "testdata", "fuzz", fuzzTarget)
	
	// Copy corpus to cache
	cacheTargetDir := filepath.Join(cacheDir, fuzzTarget)
	if err := c.fs.MkdirAll(cacheTargetDir, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create cache dir: %w", err)
	}
	
	// Copy all files from corpus
	corpusFiles, err := c.fs.ReadDir(fuzzTargetDir)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read corpus dir: %w", err)
	}
	
	numInputs := 0
	for _, f := range corpusFiles {
		if f.IsDir() {
			continue
		}
		src := filepath.Join(fuzzTargetDir, f.Name())
		dst := filepath.Join(cacheTargetDir, f.Name())
		if err := c.copyFile(src, dst); err != nil {
			return "", 0, fmt.Errorf("failed to copy corpus file: %w", err)
		}
		numInputs++
	}
	
	// Create coverage directory
	coverageDir := filepath.Join(c.Config.BaseDir, "coverage", fuzzTarget)
	if err := c.fs.MkdirAll(coverageDir, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create coverage dir: %w", err)
	}
	
	// Run fuzzing to collect coverage
	env := os.Environ()
	args := []string{
		"test", "-v", "-cover",
		fmt.Sprintf("-run=^%s$", fuzzTarget),
		fmt.Sprintf("-fuzz=^%s$", fuzzTarget),
		fmt.Sprintf("-fuzztime=%dx", numInputs),
		fmt.Sprintf("-test.gocoverdir=%s", coverageDir),
		fmt.Sprintf("-test.fuzzcachedir=%s", cacheDir),
	}
	
	output, err := c.runner.Run(filepath.Join(c.Config.LNDDir, pkg), env, "go", args...)
	if err != nil {
		// Check if it's just "no tests to run" which is expected
		if !strings.Contains(string(output), "no tests to run") {
			return "", 0, fmt.Errorf("failed to run fuzz test: %w\n%s", err, output)
		}
	}
	
	return filepath.Join("coverage", fuzzTarget), numInputs, nil
}

// combineCoverageProfiles combines multiple coverage profiles into one.
func (c *CoverageCollector) combineCoverageProfiles(coverageDirs []string) ([]byte, error) {
	// Build the input directories string
	var inputDirs []string
	for _, dir := range coverageDirs {
		inputDirs = append(inputDirs, "./"+dir)
	}
	inputDirsStr := strings.Join(inputDirs, ",")
	
	// Run covdata to combine profiles
	args := []string{
		"tool", "covdata", "textfmt",
		fmt.Sprintf("-i=%s", inputDirsStr),
		"-o=-", // Output to stdout
	}
	
	output, err := c.runner.Run(c.Config.BaseDir, os.Environ(), "go", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to run go tool covdata: %w\n%s", err, output)
	}
	
	return output, nil
}

// copyFile copies a file from src to dst.
func (c *CoverageCollector) copyFile(src, dst string) error {
	source, err := c.fs.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	
	destination, err := c.fs.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	
	_, err = destination.ReadFrom(source)
	return err
}