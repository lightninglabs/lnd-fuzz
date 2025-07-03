package lndfuzz

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestParseCoverageProfile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]int
	}{
		{
			name: "basic profile",
			content: `mode: set
github.com/lightningnetwork/lnd/lnwire/lnwire.go:1.1,2.2 1 1
github.com/lightningnetwork/lnd/lnwire/lnwire.go:3.3,4.4 1 0
github.com/lightningnetwork/lnd/lnwire/lnwire.go:5.5,6.6 2 5`,
			expected: map[string]int{
				"github.com/lightningnetwork/lnd/lnwire/lnwire.go:1.1,2.2 1": 1,
				"github.com/lightningnetwork/lnd/lnwire/lnwire.go:3.3,4.4 1": 0,
				"github.com/lightningnetwork/lnd/lnwire/lnwire.go:5.5,6.6 2": 5,
			},
		},
		{
			name: "profile with empty lines",
			content: `
github.com/example/pkg/file.go:1.1,2.2 1 10

github.com/example/pkg/file.go:3.3,4.4 1 0
`,
			expected: map[string]int{
				"github.com/example/pkg/file.go:1.1,2.2 1": 10,
				"github.com/example/pkg/file.go:3.3,4.4 1": 0,
			},
		},
		{
			name:     "empty profile",
			content:  "",
			expected: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile, err := os.CreateTemp("", "coverage-*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write content
			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write content: %v", err)
			}
			tmpFile.Close()

			// Parse profile using reader
			reader := &defaultProfileReader{}
			f, err := os.Open(tmpFile.Name())
			if err != nil {
				t.Fatalf("Failed to open file: %v", err)
			}
			defer f.Close()

			result, err := reader.ReadProfile(f)
			if err != nil {
				t.Fatalf("ReadProfile failed: %v", err)
			}

			// Compare results
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Result mismatch.\nExpected: %v\nGot: %v", tt.expected, result)
			}
		})
	}
}

func TestCompareCoverageProfiles(t *testing.T) {
	profile1 := `mode: set
github.com/example/pkg/file.go:1.1,2.2 1 1
github.com/example/pkg/file.go:3.3,4.4 1 0
github.com/example/pkg/file.go:5.5,6.6 1 3`

	profile2 := `mode: set
github.com/example/pkg/file.go:1.1,2.2 1 2
github.com/example/pkg/file.go:3.3,4.4 1 5
github.com/example/pkg/file.go:5.5,6.6 1 0
github.com/example/pkg/file.go:7.7,8.8 1 10`

	// Create temp files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "profile1.txt")
	file2 := filepath.Join(tmpDir, "profile2.txt")

	if err := os.WriteFile(file1, []byte(profile1), 0644); err != nil {
		t.Fatalf("Failed to write profile1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(profile2), 0644); err != nil {
		t.Fatalf("Failed to write profile2: %v", err)
	}

	// Compare profiles
	diff, err := CompareCoverageProfiles(file1, file2)
	if err != nil {
		t.Fatalf("CompareCoverageProfiles failed: %v", err)
	}

	// Expected newly hit blocks:
	// - line 3.3,4.4 was 0 in profile1, now 5 in profile2
	// - line 7.7,8.8 is new in profile2
	expectedBlocks := []CoverageBlock{
		{Block: "github.com/example/pkg/file.go:3.3,4.4 1", HitCount: 5},
		{Block: "github.com/example/pkg/file.go:7.7,8.8 1", HitCount: 10},
	}

	if len(diff.NewlyHitBlocks) != len(expectedBlocks) {
		t.Errorf("Expected %d newly hit blocks, got %d",
			len(expectedBlocks), len(diff.NewlyHitBlocks))
	}

	// Check blocks (order may vary)
	blockMap := make(map[string]int)
	for _, block := range diff.NewlyHitBlocks {
		blockMap[block.Block] = block.HitCount
	}

	for _, expected := range expectedBlocks {
		if hitCount, ok := blockMap[expected.Block]; !ok || hitCount != expected.HitCount {
			t.Errorf("Expected block %s with hit count %d, got %d",
				expected.Block, expected.HitCount, hitCount)
		}
	}
}

func TestWriteProfileDiff(t *testing.T) {
	diff := &ProfileDiff{
		NewlyHitBlocks: []CoverageBlock{
			{Block: "github.com/example/pkg/file.go:1.1,2.2 1", HitCount: 5},
			{Block: "github.com/example/pkg/file.go:3.3,4.4 1", HitCount: 10},
		},
	}

	tmpFile := filepath.Join(t.TempDir(), "diff.txt")

	if err := WriteProfileDiff(diff, tmpFile); err != nil {
		t.Fatalf("WriteProfileDiff failed: %v", err)
	}

	// Read back and verify
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	// Verify format
	for _, line := range lines {
		if !strings.Contains(line, "github.com/example/pkg/file.go") {
			t.Errorf("Line doesn't contain expected path: %s", line)
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			t.Errorf("Line doesn't have expected format: %s", line)
		}
	}
}

func TestProfileDiffMethods(t *testing.T) {
	diff := &ProfileDiff{
		NewlyHitBlocks: []CoverageBlock{
			{Block: "block1", HitCount: 5},
			{Block: "block2", HitCount: 10},
			{Block: "block3", HitCount: 3},
		},
	}

	if count := diff.GetNewlyHitBlocksCount(); count != 3 {
		t.Errorf("GetNewlyHitBlocksCount: expected 3, got %d", count)
	}

	if total := diff.GetTotalNewHits(); total != 18 {
		t.Errorf("GetTotalNewHits: expected 18, got %d", total)
	}
}

// Property-based test for coverage profile parsing and comparison
func TestCoverageProfileProperties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random coverage data
		numBlocks := rapid.IntRange(0, 100).Draw(t, "numBlocks")
		blocks := make(map[string]int)

		var profile1Lines []string
		var profile2Lines []string
		profile1Lines = append(profile1Lines, "mode: set")
		profile2Lines = append(profile2Lines, "mode: set")

		newlyHitCount := 0

		for i := 0; i < numBlocks; i++ {
			// Generate a block identifier
			pkg := fmt.Sprintf("github.com/example/pkg%d/file.go", rapid.IntRange(1, 5).Draw(t, "pkg"))
			startLine := rapid.IntRange(1, 1000).Draw(t, "startLine")
			startCol := rapid.IntRange(1, 100).Draw(t, "startCol")
			endLine := startLine + rapid.IntRange(0, 10).Draw(t, "endLine")
			endCol := rapid.IntRange(1, 100).Draw(t, "endCol")
			stmts := rapid.IntRange(1, 10).Draw(t, "stmts")

			block := fmt.Sprintf("%s:%d.%d,%d.%d %d", pkg, startLine, startCol, endLine, endCol, stmts)

			// Generate hit counts for both profiles
			hit1 := rapid.IntRange(0, 100).Draw(t, "hit1")
			hit2 := rapid.IntRange(0, 100).Draw(t, "hit2")

			profile1Lines = append(profile1Lines, fmt.Sprintf("%s %d", block, hit1))
			profile2Lines = append(profile2Lines, fmt.Sprintf("%s %d", block, hit2))

			blocks[block] = hit1

			// Track newly hit blocks
			if hit1 == 0 && hit2 > 0 {
				newlyHitCount++
			}
		}

		// Write profiles to temp files
		tmpDir, err := os.MkdirTemp("", "rapid-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		file1 := filepath.Join(tmpDir, "profile1.txt")
		file2 := filepath.Join(tmpDir, "profile2.txt")

		if err := os.WriteFile(file1, []byte(strings.Join(profile1Lines, "\n")), 0644); err != nil {
			t.Fatalf("Failed to write profile1: %v", err)
		}
		if err := os.WriteFile(file2, []byte(strings.Join(profile2Lines, "\n")), 0644); err != nil {
			t.Fatalf("Failed to write profile2: %v", err)
		}

		// Compare profiles
		diff, err := CompareCoverageProfiles(file1, file2)
		if err != nil {
			t.Fatalf("CompareCoverageProfiles failed: %v", err)
		}

		// Property 1: Number of newly hit blocks should match our count
		if len(diff.NewlyHitBlocks) != newlyHitCount {
			t.Errorf("Expected %d newly hit blocks, got %d", newlyHitCount, len(diff.NewlyHitBlocks))
		}

		// Property 2: All newly hit blocks should have been 0 in profile1
		reader := &defaultProfileReader{}
		f1, err := os.Open(file1)
		if err != nil {
			t.Fatalf("Failed to open profile1: %v", err)
		}
		defer f1.Close()

		profile1Data, err := reader.ReadProfile(f1)
		if err != nil {
			t.Fatalf("Failed to parse profile1: %v", err)
		}

		for _, block := range diff.NewlyHitBlocks {
			if profile1Data[block.Block] != 0 {
				t.Errorf("Block %s was not 0 in profile1 (was %d)", block.Block, profile1Data[block.Block])
			}
			if block.HitCount <= 0 {
				t.Errorf("Block %s has non-positive hit count in profile2: %d", block.Block, block.HitCount)
			}
		}

		// Property 3: Writing and reading back should preserve data
		outputFile := filepath.Join(tmpDir, "diff.txt")
		if err := WriteProfileDiff(diff, outputFile); err != nil {
			t.Fatalf("WriteProfileDiff failed: %v", err)
		}

		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if string(content) != "" && len(lines) != len(diff.NewlyHitBlocks) {
			t.Errorf("Output has %d lines, expected %d", len(lines), len(diff.NewlyHitBlocks))
		}
	})
}
