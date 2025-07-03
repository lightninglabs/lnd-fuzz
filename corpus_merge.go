package lndfuzz

import "fmt"

// CorpusMergeConfig contains configuration for corpus merging.
type CorpusMergeConfig struct {
	DestDir     string
	SrcDir      string
	PackageDir  string
	FuzzTarget  string
	CacheDir    string
	
	// Optional fields for dependency injection
	FS            FileSystem
	CmdRunner     CommandRunner
}

// FileInfo holds file information for sorting by size.
type FileInfo struct {
	Name string
	Size int64
}

// MergeCorpus is a backward compatibility wrapper for the new CorpusMerger.
// It merges new fuzzing inputs into an existing corpus, only keeping
// inputs that increase coverage as measured by Go's native fuzzing engine.
// Prefers smaller inputs over larger ones.
//
// Deprecated: Use NewCorpusMerger for more control and better testability.
func MergeCorpus(cfg CorpusMergeConfig) error {
	merger := NewCorpusMerger(cfg)
	
	// Use a simple console reporter
	merger.SetProgressReporter(&consoleReporter{})
	
	// Analyze
	result, err := merger.Analyze()
	if err != nil {
		return err
	}
	
	// Apply
	if err := merger.Apply(result); err != nil {
		return err
	}
	
	fmt.Printf("\nAdded %d new inputs. Final coverage: %d\n", 
		result.InputsAdded, result.FinalCoverage)
	
	return nil
}

// consoleReporter implements ProgressReporter for console output.
type consoleReporter struct{}

func (c *consoleReporter) ReportProgress(current, total int, message string) {
	fmt.Printf("\r\033[2KMeasuring coverage for input %d/%d", current, total)
}

func (c *consoleReporter) ReportInfo(message string) {
	fmt.Println(message)
}

func (c *consoleReporter) ReportWarning(message string) {
	fmt.Printf("Warning: %s\n", message)
}

func (c *consoleReporter) ReportError(message string) {
	fmt.Printf("Error: %s\n", message)
}