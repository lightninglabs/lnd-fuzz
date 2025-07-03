package lndfuzz

import "time"

// CoverageTarget represents coverage data for a single fuzz target.
type CoverageTarget struct {
	Package     string
	Target      string
	NumInputs   int
	CoverageDir string
	Error       error
}

// CoverageResult contains the results of coverage collection.
type CoverageResult struct {
	Targets         []CoverageTarget
	CombinedProfile []byte // The actual coverage profile data
	ProfilePath     string // Where it would be written
	StartTime       time.Time
	EndTime         time.Time
}