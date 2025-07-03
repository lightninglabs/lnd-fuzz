package lndfuzz

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CorpusMerger handles the logic for merging fuzzing corpora.
type CorpusMerger struct {
	Config   CorpusMergeConfig
	fs       FileSystem
	runner   CommandRunner
	progress ProgressReporter
}

// NewCorpusMerger creates a new CorpusMerger with the given configuration.
func NewCorpusMerger(cfg CorpusMergeConfig) *CorpusMerger {
	// Set defaults
	if cfg.FS == nil {
		cfg.FS = OSFileSystem{}
	}
	if cfg.CmdRunner == nil {
		cfg.CmdRunner = &OSCommandRunner{}
	}

	return &CorpusMerger{
		Config:   cfg,
		fs:       cfg.FS,
		runner:   cfg.CmdRunner,
		progress: NullProgressReporter{},
	}
}

// SetProgressReporter sets the progress reporter for this merger.
func (m *CorpusMerger) SetProgressReporter(reporter ProgressReporter) {
	if reporter != nil {
		m.progress = reporter
	} else {
		m.progress = NullProgressReporter{}
	}
}

// Analyze performs the corpus merge analysis without modifying any files.
// It returns information about what would be merged.
func (m *CorpusMerger) Analyze() (*MergeResult, error) {
	result := &MergeResult{
		StartTime: time.Now(),
		Inputs:    []MergeInput{},
	}

	// Create cache directory if not provided
	cacheDir := m.Config.CacheDir
	if cacheDir == "" {
		tmpDir, err := m.fs.TempDir("", "fuzz-merge-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer m.fs.RemoveAll(tmpDir)
		cacheDir = tmpDir
	}

	// Validate arguments
	if err := m.validateArgs(); err != nil {
		return nil, err
	}

	// Create fuzz target cache directory
	targetCacheDir := filepath.Join(cacheDir, m.Config.FuzzTarget)
	if err := m.fs.MkdirAll(targetCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Handle testdata directory
	fuzzTestdataDir := filepath.Join(m.Config.PackageDir, "testdata", "fuzz", m.Config.FuzzTarget)
	backupDir := fuzzTestdataDir + ".bak"

	if _, err := m.fs.Stat(fuzzTestdataDir); err == nil {
		if err := m.fs.Rename(fuzzTestdataDir, backupDir); err != nil {
			return nil, fmt.Errorf("failed to backup testdata: %w", err)
		}
		defer func() {
			// Restore backup
			m.fs.Rename(backupDir, fuzzTestdataDir)
		}()
	}

	// Measure baseline coverage
	coverage := 0
	destFiles, err := m.fs.ReadDir(m.Config.DestDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read dest dir: %w", err)
	}

	if len(destFiles) > 0 {
		// Copy existing corpus to cache
		for _, f := range destFiles {
			if f.IsDir() {
				continue
			}
			src := filepath.Join(m.Config.DestDir, f.Name())
			dst := filepath.Join(targetCacheDir, f.Name())
			if err := m.copyFile(src, dst); err != nil {
				return nil, fmt.Errorf("failed to copy %s: %w", f.Name(), err)
			}
		}

		coverage, err = m.measureCoverage(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to measure baseline coverage: %w", err)
		}
	}

	result.BaselineCoverage = coverage
	m.progress.ReportInfo(fmt.Sprintf("Baseline coverage: %d", coverage))

	// Get sorted list of source files
	srcFiles, err := m.getSortedFiles(m.Config.SrcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read source dir: %w", err)
	}

	result.InputsAnalyzed = len(srcFiles)

	// Analyze each input
	for i, f := range srcFiles {
		m.progress.ReportProgress(i+1, len(srcFiles),
			fmt.Sprintf("Analyzing %s", f.Name))

		input := MergeInput{
			Name: f.Name,
			Size: f.Size,
		}

		// Check if already in destination
		destPath := filepath.Join(m.Config.DestDir, f.Name)
		if _, err := m.fs.Stat(destPath); err == nil {
			input.SkippedReason = "already exists"
			result.Inputs = append(result.Inputs, input)
			continue
		}

		// Copy to cache and measure coverage
		srcPath := filepath.Join(m.Config.SrcDir, f.Name)
		cachePath := filepath.Join(targetCacheDir, f.Name)
		if err := m.copyFile(srcPath, cachePath); err != nil {
			return nil, fmt.Errorf("failed to copy %s to cache: %w", f.Name, err)
		}

		newCoverage, err := m.measureCoverage(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to measure coverage for %s: %w", f.Name, err)
		}

		if newCoverage > coverage {
			input.CoverageIncrease = newCoverage - coverage
			input.Added = true
			coverage = newCoverage
			result.InputsAdded++
			m.progress.ReportInfo(fmt.Sprintf("Input %s increased coverage by %d to %d",
				f.Name, input.CoverageIncrease, coverage))
		} else {
			// Remove from cache if no coverage increase
			m.fs.Remove(cachePath)
			input.SkippedReason = "no coverage increase"

			if newCoverage < coverage {
				m.progress.ReportWarning(fmt.Sprintf(
					"Nondeterministic fuzz target: coverage decreased from %d to %d",
					coverage, newCoverage))
			}
		}

		result.Inputs = append(result.Inputs, input)
	}

	result.FinalCoverage = coverage
	result.EndTime = time.Now()

	return result, nil
}

// Apply applies the merge results by copying the files that should be added.
func (m *CorpusMerger) Apply(result *MergeResult) error {
	if result == nil {
		return fmt.Errorf("no merge result provided")
	}

	copiedCount := 0
	for _, input := range result.Inputs {
		if !input.Added {
			continue
		}

		srcPath := filepath.Join(m.Config.SrcDir, input.Name)
		destPath := filepath.Join(m.Config.DestDir, input.Name)

		if err := m.copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("failed to copy %s: %w", input.Name, err)
		}
		copiedCount++
	}

	if copiedCount != result.InputsAdded {
		return fmt.Errorf("expected to copy %d files but copied %d",
			result.InputsAdded, copiedCount)
	}

	return nil
}

// validateArgs validates the merge configuration.
func (m *CorpusMerger) validateArgs() error {
	fuzzTestdataDir := filepath.Join(m.Config.PackageDir, "testdata", "fuzz", m.Config.FuzzTarget)
	backupDir := fuzzTestdataDir + ".bak"

	if _, err := m.fs.Stat(backupDir); err == nil {
		return fmt.Errorf("%s already exists", backupDir)
	}

	// Check if paths are the same
	if m.sameFile(fuzzTestdataDir, m.Config.SrcDir) {
		return fmt.Errorf("SRC_DIR must not be the testdata fuzz seed directory")
	}
	if m.sameFile(fuzzTestdataDir, m.Config.DestDir) {
		return fmt.Errorf("DEST_DIR must not be the testdata fuzz seed directory")
	}

	return nil
}

// sameFile checks if two paths refer to the same file.
func (m *CorpusMerger) sameFile(path1, path2 string) bool {
	abs1, err1 := filepath.Abs(path1)
	abs2, err2 := filepath.Abs(path2)
	return err1 == nil && err2 == nil && abs1 == abs2
}

// measureCoverage runs the fuzz test and extracts the coverage measurement.
func (m *CorpusMerger) measureCoverage(cacheDir string) (int, error) {
	// Count inputs
	targetCacheDir := filepath.Join(cacheDir, m.Config.FuzzTarget)
	inputs, err := m.fs.ReadDir(targetCacheDir)
	if err != nil {
		return 0, err
	}
	numInputs := 0
	for _, f := range inputs {
		if !f.IsDir() {
			numInputs++
		}
	}

	// Run fuzzing with debug output
	env := append(os.Environ(), "GODEBUG=fuzzdebug=1")
	args := []string{
		"test",
		fmt.Sprintf("-run=^%s$", m.Config.FuzzTarget),
		fmt.Sprintf("-fuzz=^%s$", m.Config.FuzzTarget),
		fmt.Sprintf("-fuzztime=%dx", numInputs),
		fmt.Sprintf("-test.fuzzcachedir=%s", cacheDir),
	}

	output, err := m.runner.Run(m.Config.PackageDir, env, "go", args...)
	if err != nil {
		// Check if the error is due to no tests found (expected)
		if !strings.Contains(string(output), "no tests to run") {
			return 0, fmt.Errorf("go test failed: %w\n%s", err, output)
		}
	}

	// Extract coverage bits
	re := regexp.MustCompile(`initial coverage bits:\s*(\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not find coverage bits in output")
	}

	coverage, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("failed to parse coverage: %w", err)
	}

	return coverage, nil
}

// getSortedFiles returns files sorted by size (smallest first).
func (m *CorpusMerger) getSortedFiles(dir string) ([]FileInfo, error) {
	entries, err := m.fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name: e.Name(),
			Size: info.Size(),
		})
	}

	// Sort by size (smallest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Size < files[j].Size
	})

	return files, nil
}

// copyFile copies a file from src to dst.
func (m *CorpusMerger) copyFile(src, dst string) error {
	source, err := m.fs.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Ensure destination directory exists
	if err := m.fs.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destination, err := m.fs.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = destination.ReadFrom(source)
	return err
}
