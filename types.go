package lndfuzz

import "time"

// MergeInput represents a single input that was analyzed during corpus merge.
type MergeInput struct {
	Name             string
	Size             int64
	CoverageIncrease int
	Added            bool
	SkippedReason    string // e.g., "already exists", "no coverage increase"
}

// MergeResult contains the results of a corpus merge operation.
type MergeResult struct {
	BaselineCoverage int
	FinalCoverage    int
	InputsAnalyzed   int
	InputsAdded      int
	Inputs           []MergeInput
	StartTime        time.Time
	EndTime          time.Time
}

// CoverageTarget represents coverage data for a single fuzz target.
type CoverageTarget struct {
	Package    string
	Target     string
	NumInputs  int
	CoverageDir string
	Error      error
}

// CoverageResult contains the results of coverage collection.
type CoverageResult struct {
	Targets         []CoverageTarget
	CombinedProfile []byte // The actual coverage profile data
	ProfilePath     string // Where it would be written
	StartTime       time.Time
	EndTime         time.Time
}

// ComparisonResult contains the results of comparing two coverage profiles.
type ComparisonResult struct {
	NewlyHitBlocks []CoverageBlock
	TotalNewBlocks int
	TotalNewHits   int
	Profile1Path   string
	Profile2Path   string
}

// ProgressReporter is an interface for reporting progress during long operations.
type ProgressReporter interface {
	// ReportProgress is called periodically during operations.
	// current is the current item being processed, total is the total number of items.
	// message provides additional context about what's happening.
	ReportProgress(current, total int, message string)
	
	// ReportInfo reports general information messages.
	ReportInfo(message string)
	
	// ReportWarning reports warning messages.
	ReportWarning(message string)
	
	// ReportError reports error messages that don't stop execution.
	ReportError(message string)
}

// NullProgressReporter is a no-op implementation of ProgressReporter.
type NullProgressReporter struct{}

func (NullProgressReporter) ReportProgress(current, total int, message string) {}
func (NullProgressReporter) ReportInfo(message string) {}
func (NullProgressReporter) ReportWarning(message string) {}
func (NullProgressReporter) ReportError(message string) {}