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