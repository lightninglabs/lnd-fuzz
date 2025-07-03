package lndfuzz

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ProfileComparator handles comparison of coverage profiles.
type ProfileComparator struct {
	// Optional dependency injection
	profileReader CoverageProfileReader
}

// NewProfileComparator creates a new ProfileComparator.
func NewProfileComparator() *ProfileComparator {
	return &ProfileComparator{
		profileReader: &defaultProfileReader{},
	}
}

// SetProfileReader sets a custom profile reader.
func (p *ProfileComparator) SetProfileReader(reader CoverageProfileReader) {
	if reader != nil {
		p.profileReader = reader
	}
}

// Compare compares two coverage profiles and returns the differences.
func (p *ProfileComparator) Compare(firstProfilePath, secondProfilePath string) (*ComparisonResult, error) {
	// Open first profile
	f1, err := os.Open(firstProfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open first profile: %w", err)
	}
	defer f1.Close()

	// Open second profile
	f2, err := os.Open(secondProfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open second profile: %w", err)
	}
	defer f2.Close()

	// Parse profiles
	firstCoverage, err := p.profileReader.ReadProfile(f1)
	if err != nil {
		return nil, fmt.Errorf("failed to parse first profile: %w", err)
	}

	secondCoverage, err := p.profileReader.ReadProfile(f2)
	if err != nil {
		return nil, fmt.Errorf("failed to parse second profile: %w", err)
	}

	// Find newly hit blocks
	var newlyHitBlocks []CoverageBlock
	for block, hitCount := range secondCoverage {
		if hitCount > 0 && firstCoverage[block] == 0 {
			newlyHitBlocks = append(newlyHitBlocks, CoverageBlock{
				Block:    block,
				HitCount: hitCount,
			})
		}
	}

	// Calculate totals
	totalNewHits := 0
	for _, block := range newlyHitBlocks {
		totalNewHits += block.HitCount
	}

	return &ComparisonResult{
		NewlyHitBlocks: newlyHitBlocks,
		TotalNewBlocks: len(newlyHitBlocks),
		TotalNewHits:   totalNewHits,
		Profile1Path:   firstProfilePath,
		Profile2Path:   secondProfilePath,
	}, nil
}

// defaultProfileReader implements the default coverage profile parsing.
type defaultProfileReader struct{}

// ReadProfile reads a coverage profile from the given reader.
func (r *defaultProfileReader) ReadProfile(reader io.Reader) (map[string]int, error) {
	coverage := make(map[string]int)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Split on whitespace
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		// The last part should be the hit count
		lastPart := parts[len(parts)-1]
		hitCount, err := strconv.Atoi(lastPart)
		if err != nil {
			// This line doesn't end with a number, skip it
			continue
		}

		// Everything before the hit count is the block
		block := strings.Join(parts[:len(parts)-1], " ")
		coverage[block] = hitCount
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan file: %w", err)
	}

	return coverage, nil
}

// CompareCoverageProfiles is a convenience function using the new architecture.
func CompareCoverageProfiles(firstProfilePath, secondProfilePath string) (*ProfileDiff, error) {
	comparator := NewProfileComparator()
	result, err := comparator.Compare(firstProfilePath, secondProfilePath)
	if err != nil {
		return nil, err
	}

	// Convert to legacy ProfileDiff for backward compatibility
	return &ProfileDiff{
		NewlyHitBlocks: result.NewlyHitBlocks,
	}, nil
}

// WriteProfileDiff writes the comparison result to a file.
func WriteProfileDiff(diff *ProfileDiff, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for _, block := range diff.NewlyHitBlocks {
		if _, err := fmt.Fprintf(writer, "%s %d\n", block.Block, block.HitCount); err != nil {
			return fmt.Errorf("failed to write block: %w", err)
		}
	}

	return nil
}

// WriteComparisonResult writes the comparison result to a file.
func WriteComparisonResult(result *ComparisonResult, outputPath string) error {
	// Convert to ProfileDiff and use existing writer
	diff := &ProfileDiff{
		NewlyHitBlocks: result.NewlyHitBlocks,
	}
	return WriteProfileDiff(diff, outputPath)
}
