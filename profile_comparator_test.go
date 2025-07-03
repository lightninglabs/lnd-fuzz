package lndfuzz

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// mockProfileReader is a mock implementation of CoverageProfileReader for testing.
type mockProfileReader struct {
	profiles map[string]map[string]int
	err      error
}

func (m *mockProfileReader) ReadProfile(r io.Reader) (map[string]int, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Get filename from the reader if it's a file
	if f, ok := r.(*os.File); ok {
		name := f.Name()
		// Check if filename ends with one of our profile names
		for key, profile := range m.profiles {
			if strings.HasSuffix(name, key) {
				return profile, nil
			}
		}
		// Also check for exact match
		if profile, exists := m.profiles[name]; exists {
			return profile, nil
		}
	}

	// Default empty profile
	return map[string]int{}, nil
}

func TestProfileComparator_Compare(t *testing.T) {
	tests := []struct {
		name           string
		setupFiles     func(string) (string, string)
		setupReader    func() *mockProfileReader
		expectError    bool
		errorContains  string
		validateResult func(*ComparisonResult) error
	}{
		{
			name: "successful comparison with new blocks",
			setupFiles: func(tmpDir string) (string, string) {
				profile1 := filepath.Join(tmpDir, "profile1.txt")
				profile2 := filepath.Join(tmpDir, "profile2.txt")

				os.WriteFile(profile1, []byte(`mode: set
github.com/example/file.go:1.1,2.2 1 10
github.com/example/file.go:3.3,4.4 1 0
github.com/example/file.go:5.5,6.6 1 5`), 0644)

				os.WriteFile(profile2, []byte(`mode: set
github.com/example/file.go:1.1,2.2 1 15
github.com/example/file.go:3.3,4.4 1 20
github.com/example/file.go:5.5,6.6 1 0
github.com/example/file.go:7.7,8.8 1 30`), 0644)

				return profile1, profile2
			},
			validateResult: func(result *ComparisonResult) error {
				if len(result.NewlyHitBlocks) != 2 {
					return fmt.Errorf("expected 2 newly hit blocks, got %d", len(result.NewlyHitBlocks))
				}

				// Check for specific blocks
				foundBlock3 := false
				foundBlock7 := false
				for _, block := range result.NewlyHitBlocks {
					if strings.Contains(block.Block, "3.3,4.4") && block.HitCount == 20 {
						foundBlock3 = true
					}
					if strings.Contains(block.Block, "7.7,8.8") && block.HitCount == 30 {
						foundBlock7 = true
					}
				}

				if !foundBlock3 || !foundBlock7 {
					return fmt.Errorf("expected blocks not found")
				}

				if result.TotalNewBlocks != 2 {
					return fmt.Errorf("expected TotalNewBlocks=2, got %d", result.TotalNewBlocks)
				}

				if result.TotalNewHits != 50 {
					return fmt.Errorf("expected TotalNewHits=50, got %d", result.TotalNewHits)
				}

				return nil
			},
		},
		{
			name: "no new blocks",
			setupFiles: func(tmpDir string) (string, string) {
				profile1 := filepath.Join(tmpDir, "profile1.txt")
				profile2 := filepath.Join(tmpDir, "profile2.txt")

				content := `mode: set
github.com/example/file.go:1.1,2.2 1 10
github.com/example/file.go:3.3,4.4 1 5`

				os.WriteFile(profile1, []byte(content), 0644)
				os.WriteFile(profile2, []byte(content), 0644)

				return profile1, profile2
			},
			validateResult: func(result *ComparisonResult) error {
				if len(result.NewlyHitBlocks) != 0 {
					return fmt.Errorf("expected no newly hit blocks, got %d", len(result.NewlyHitBlocks))
				}
				if result.TotalNewBlocks != 0 {
					return fmt.Errorf("expected TotalNewBlocks=0, got %d", result.TotalNewBlocks)
				}
				if result.TotalNewHits != 0 {
					return fmt.Errorf("expected TotalNewHits=0, got %d", result.TotalNewHits)
				}
				return nil
			},
		},
		{
			name: "empty profiles",
			setupFiles: func(tmpDir string) (string, string) {
				profile1 := filepath.Join(tmpDir, "profile1.txt")
				profile2 := filepath.Join(tmpDir, "profile2.txt")

				os.WriteFile(profile1, []byte(""), 0644)
				os.WriteFile(profile2, []byte(""), 0644)

				return profile1, profile2
			},
			validateResult: func(result *ComparisonResult) error {
				if len(result.NewlyHitBlocks) != 0 {
					return fmt.Errorf("expected no newly hit blocks")
				}
				return nil
			},
		},
		{
			name: "file not found - first profile",
			setupFiles: func(tmpDir string) (string, string) {
				profile2 := filepath.Join(tmpDir, "profile2.txt")
				os.WriteFile(profile2, []byte("mode: set"), 0644)
				return filepath.Join(tmpDir, "nonexistent1.txt"), profile2
			},
			expectError:   true,
			errorContains: "failed to open first profile",
		},
		{
			name: "file not found - second profile",
			setupFiles: func(tmpDir string) (string, string) {
				profile1 := filepath.Join(tmpDir, "profile1.txt")
				os.WriteFile(profile1, []byte("mode: set"), 0644)
				return profile1, filepath.Join(tmpDir, "nonexistent2.txt")
			},
			expectError:   true,
			errorContains: "failed to open second profile",
		},
		{
			name: "all blocks in second profile are new",
			setupFiles: func(tmpDir string) (string, string) {
				profile1 := filepath.Join(tmpDir, "profile1.txt")
				profile2 := filepath.Join(tmpDir, "profile2.txt")

				os.WriteFile(profile1, []byte("mode: set\n"), 0644)
				os.WriteFile(profile2, []byte(`mode: set
github.com/example/file.go:1.1,2.2 1 10
github.com/example/file.go:3.3,4.4 1 20`), 0644)

				return profile1, profile2
			},
			validateResult: func(result *ComparisonResult) error {
				if len(result.NewlyHitBlocks) != 2 {
					return fmt.Errorf("expected 2 newly hit blocks, got %d", len(result.NewlyHitBlocks))
				}
				if result.TotalNewHits != 30 {
					return fmt.Errorf("expected TotalNewHits=30, got %d", result.TotalNewHits)
				}
				return nil
			},
		},
		{
			name: "increased coverage for existing blocks doesn't count as new",
			setupFiles: func(tmpDir string) (string, string) {
				profile1 := filepath.Join(tmpDir, "profile1.txt")
				profile2 := filepath.Join(tmpDir, "profile2.txt")

				os.WriteFile(profile1, []byte(`mode: set
github.com/example/file.go:1.1,2.2 1 10`), 0644)

				os.WriteFile(profile2, []byte(`mode: set
github.com/example/file.go:1.1,2.2 1 20`), 0644)

				return profile1, profile2
			},
			validateResult: func(result *ComparisonResult) error {
				if len(result.NewlyHitBlocks) != 0 {
					return fmt.Errorf("expected no newly hit blocks, got %d", len(result.NewlyHitBlocks))
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			comparator := NewProfileComparator()

			// Setup custom reader if provided
			if tt.setupReader != nil {
				comparator.SetProfileReader(tt.setupReader())
			}

			// Setup files
			var profile1, profile2 string
			if tt.setupFiles != nil {
				profile1, profile2 = tt.setupFiles(tmpDir)
			}

			// Compare profiles
			result, err := comparator.Compare(profile1, profile2)

			// Check error
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing '%s', got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate result
			if tt.validateResult != nil {
				if err := tt.validateResult(result); err != nil {
					t.Errorf("result validation failed: %v", err)
				}
			}

			// Verify paths are set correctly
			if result.Profile1Path != profile1 {
				t.Errorf("expected Profile1Path=%s, got %s", profile1, result.Profile1Path)
			}
			if result.Profile2Path != profile2 {
				t.Errorf("expected Profile2Path=%s, got %s", profile2, result.Profile2Path)
			}
		})
	}
}

func TestProfileComparator_SetProfileReader(t *testing.T) {
	comparator := NewProfileComparator()

	// Test setting a custom reader
	customReader := &mockProfileReader{
		profiles: map[string]map[string]int{},
	}
	comparator.SetProfileReader(customReader)

	// Verify it uses the custom reader
	if comparator.profileReader != customReader {
		t.Errorf("custom reader was not set")
	}

	// Test setting nil reader (should keep existing)
	originalReader := comparator.profileReader
	comparator.SetProfileReader(nil)
	if comparator.profileReader != originalReader {
		t.Errorf("nil reader should not replace existing reader")
	}
}

func TestProfileComparator_WithMockReader(t *testing.T) {
	tmpDir := t.TempDir()
	profile1 := filepath.Join(tmpDir, "profile1.txt")
	profile2 := filepath.Join(tmpDir, "profile2.txt")

	// Create dummy files
	os.WriteFile(profile1, []byte("dummy"), 0644)
	os.WriteFile(profile2, []byte("dummy"), 0644)

	// Setup mock reader with predefined profiles
	mockReader := &mockProfileReader{
		profiles: map[string]map[string]int{
			profile1: {
				"block1": 10,
				"block2": 0,
			},
			profile2: {
				"block1": 15,
				"block2": 5,
				"block3": 20,
			},
		},
	}

	comparator := NewProfileComparator()
	comparator.SetProfileReader(mockReader)

	result, err := comparator.Compare(profile1, profile2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find block2 and block3 as newly hit
	if len(result.NewlyHitBlocks) != 2 {
		t.Errorf("expected 2 newly hit blocks, got %d", len(result.NewlyHitBlocks))
	}

	// Check total hits
	if result.TotalNewHits != 25 { // 5 + 20
		t.Errorf("expected TotalNewHits=25, got %d", result.TotalNewHits)
	}
}

func TestProfileComparator_ReaderError(t *testing.T) {
	tmpDir := t.TempDir()
	profile1 := filepath.Join(tmpDir, "profile1.txt")
	profile2 := filepath.Join(tmpDir, "profile2.txt")

	// Create dummy files
	os.WriteFile(profile1, []byte("dummy"), 0644)
	os.WriteFile(profile2, []byte("dummy"), 0644)

	// Test error on first profile
	mockReader := &mockProfileReader{
		err: errors.New("read error"),
	}

	comparator := NewProfileComparator()
	comparator.SetProfileReader(mockReader)

	_, err := comparator.Compare(profile1, profile2)
	if err == nil {
		t.Errorf("expected error but got none")
	}
	if !strings.Contains(err.Error(), "failed to parse first profile") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

// Property-based test for ProfileComparator
func TestProfileComparatorProperties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random profiles
		numBlocks := rapid.IntRange(0, 50).Draw(t, "numBlocks")

		profile1 := make(map[string]int)
		profile2 := make(map[string]int)

		expectedNewBlocks := 0
		expectedNewHits := 0

		for i := 0; i < numBlocks; i++ {
			// Use index to ensure unique blocks
			block := fmt.Sprintf("file.go:%d.%d,%d.%d %d",
				i+1, // startLine based on index to ensure uniqueness
				rapid.IntRange(1, 100).Draw(t, "startCol"),
				i+1, // endLine
				rapid.IntRange(1, 100).Draw(t, "endCol"),
				rapid.IntRange(1, 5).Draw(t, "stmts"))

			// Generate hit counts
			hits1 := rapid.IntRange(0, 100).Draw(t, "hits1")
			hits2 := rapid.IntRange(0, 100).Draw(t, "hits2")

			profile1[block] = hits1
			profile2[block] = hits2

			// Track expected newly hit blocks
			if hits1 == 0 && hits2 > 0 {
				expectedNewBlocks++
				expectedNewHits += hits2
			}
		}

		// Create mock reader
		mockReader := &mockProfileReader{
			profiles: map[string]map[string]int{
				// Use any key that ends with the right suffix
				"profile1": profile1,
				"profile2": profile2,
			},
		}

		comparator := NewProfileComparator()
		comparator.SetProfileReader(mockReader)

		// Create dummy files
		tmpDir, _ := os.MkdirTemp("", "test-*")
		defer os.RemoveAll(tmpDir)

		p1 := filepath.Join(tmpDir, "profile1")
		p2 := filepath.Join(tmpDir, "profile2")
		os.WriteFile(p1, []byte("dummy"), 0644)
		os.WriteFile(p2, []byte("dummy"), 0644)

		result, err := comparator.Compare(p1, p2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Debug output on failure
		if result.TotalNewBlocks != expectedNewBlocks || result.TotalNewHits != expectedNewHits {
			t.Logf("Profile1: %v", profile1)
			t.Logf("Profile2: %v", profile2)
			t.Logf("Expected new blocks: %d, got: %d", expectedNewBlocks, result.TotalNewBlocks)
			t.Logf("Expected new hits: %d, got: %d", expectedNewHits, result.TotalNewHits)
			t.Logf("Newly hit blocks: %v", result.NewlyHitBlocks)
		}

		// Property 1: Number of newly hit blocks should match expected
		if result.TotalNewBlocks != expectedNewBlocks {
			t.Errorf("expected %d newly hit blocks, got %d", expectedNewBlocks, result.TotalNewBlocks)
		}

		// Property 2: Total new hits should match expected
		if result.TotalNewHits != expectedNewHits {
			t.Errorf("expected %d total new hits, got %d", expectedNewHits, result.TotalNewHits)
		}

		// Property 3: Length of NewlyHitBlocks should match TotalNewBlocks
		if len(result.NewlyHitBlocks) != result.TotalNewBlocks {
			t.Errorf("NewlyHitBlocks length (%d) doesn't match TotalNewBlocks (%d)",
				len(result.NewlyHitBlocks), result.TotalNewBlocks)
		}

		// Property 4: All newly hit blocks should have been 0 in profile1
		for _, block := range result.NewlyHitBlocks {
			if profile1[block.Block] != 0 {
				t.Errorf("block %s was not 0 in profile1 (was %d)", block.Block, profile1[block.Block])
			}
			if block.HitCount <= 0 {
				t.Errorf("block %s has non-positive hit count: %d", block.Block, block.HitCount)
			}
		}

		// Property 5: Sum of individual hit counts should equal TotalNewHits
		sumHits := 0
		for _, block := range result.NewlyHitBlocks {
			sumHits += block.HitCount
		}
		if sumHits != result.TotalNewHits {
			t.Errorf("sum of hit counts (%d) doesn't match TotalNewHits (%d)", sumHits, result.TotalNewHits)
		}
	})
}

func TestDefaultProfileReader_EdgeCases(t *testing.T) {
	reader := &defaultProfileReader{}

	tests := []struct {
		name     string
		content  string
		expected map[string]int
	}{
		{
			name: "malformed lines",
			content: `mode: set
github.com/example/file.go:1.1,2.2 1 10
this is not a valid line
github.com/example/file.go:3.3,4.4 1 20
another invalid line with no number at end
github.com/example/file.go:5.5,6.6 1 not_a_number
github.com/example/file.go:7.7,8.8 1 30`,
			expected: map[string]int{
				"github.com/example/file.go:1.1,2.2 1": 10,
				"github.com/example/file.go:3.3,4.4 1": 20,
				"github.com/example/file.go:7.7,8.8 1": 30,
			},
		},
		{
			name: "various whitespace",
			content: `mode: set
github.com/example/file.go:1.1,2.2 1    10
github.com/example/file.go:3.3,4.4 1	20
  github.com/example/file.go:5.5,6.6 1 30  `,
			expected: map[string]int{
				"github.com/example/file.go:1.1,2.2 1": 10,
				"github.com/example/file.go:3.3,4.4 1": 20,
				"github.com/example/file.go:5.5,6.6 1": 30,
			},
		},
		{
			name:     "only header",
			content:  "mode: set",
			expected: map[string]int{},
		},
		{
			name: "duplicate blocks (last wins)",
			content: `github.com/example/file.go:1.1,2.2 1 10
github.com/example/file.go:1.1,2.2 1 20`,
			expected: map[string]int{
				"github.com/example/file.go:1.1,2.2 1": 20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reader.ReadProfile(strings.NewReader(tt.content))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d entries, got %d", len(tt.expected), len(result))
			}

			for block, expectedHits := range tt.expected {
				if hits, ok := result[block]; !ok || hits != expectedHits {
					t.Errorf("block %s: expected %d hits, got %d (exists: %v)",
						block, expectedHits, hits, ok)
				}
			}
		})
	}
}

func TestWriteComparisonResult(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.txt")

	result := &ComparisonResult{
		NewlyHitBlocks: []CoverageBlock{
			{Block: "file.go:1.1,2.2 1", HitCount: 10},
			{Block: "file.go:3.3,4.4 1", HitCount: 20},
		},
		TotalNewBlocks: 2,
		TotalNewHits:   30,
		Profile1Path:   "profile1.txt",
		Profile2Path:   "profile2.txt",
	}

	err := WriteComparisonResult(result, outputPath)
	if err != nil {
		t.Fatalf("failed to write result: %v", err)
	}

	// Read back and verify
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	// Verify format
	expectedLines := []string{
		"file.go:1.1,2.2 1 10",
		"file.go:3.3,4.4 1 20",
	}

	for i, expected := range expectedLines {
		if i < len(lines) && lines[i] != expected {
			t.Errorf("line %d: expected '%s', got '%s'", i, expected, lines[i])
		}
	}
}

func TestCompareCoverageProfiles_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	profile1 := filepath.Join(tmpDir, "profile1.txt")
	profile2 := filepath.Join(tmpDir, "profile2.txt")

	os.WriteFile(profile1, []byte(`mode: set
github.com/example/file.go:1.1,2.2 1 0
github.com/example/file.go:3.3,4.4 1 5`), 0644)

	os.WriteFile(profile2, []byte(`mode: set
github.com/example/file.go:1.1,2.2 1 10
github.com/example/file.go:3.3,4.4 1 5`), 0644)

	// Use backward compatibility function
	diff, err := CompareCoverageProfiles(profile1, profile2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diff.NewlyHitBlocks) != 1 {
		t.Errorf("expected 1 newly hit block, got %d", len(diff.NewlyHitBlocks))
	}

	if diff.NewlyHitBlocks[0].HitCount != 10 {
		t.Errorf("expected hit count 10, got %d", diff.NewlyHitBlocks[0].HitCount)
	}
}
