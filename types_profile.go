package lndfuzz

// CoverageBlock represents a single coverage block from a Go coverage profile.
type CoverageBlock struct {
	Block    string
	HitCount int
}

// ProfileDiff represents the difference between two coverage profiles.
type ProfileDiff struct {
	NewlyHitBlocks []CoverageBlock
}

// GetNewlyHitBlocksCount returns the number of newly hit blocks in the diff.
func (d *ProfileDiff) GetNewlyHitBlocksCount() int {
	return len(d.NewlyHitBlocks)
}

// GetTotalNewHits returns the total number of new hits across all blocks.
func (d *ProfileDiff) GetTotalNewHits() int {
	total := 0
	for _, block := range d.NewlyHitBlocks {
		total += block.HitCount
	}
	return total
}

// ComparisonResult contains the results of comparing two coverage profiles.
type ComparisonResult struct {
	NewlyHitBlocks []CoverageBlock
	TotalNewBlocks int
	TotalNewHits   int
	Profile1Path   string
	Profile2Path   string
}