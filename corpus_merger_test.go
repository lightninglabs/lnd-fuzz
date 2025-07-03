package lndfuzz

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestCorpusMergerAnalyze(t *testing.T) {
	tests := []struct {
		name           string
		setupFS        func(*MockFileSystem)
		setupRunner    func(*MockCommandRunner)
		expectedResult func(*MergeResult) error
		expectError    bool
		errorContains  string
	}{
		{
			name: "empty source directory",
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/dest")
				fs.AddDir("/src")
				fs.AddDir("/pkg")
			},
			setupRunner: func(runner *MockCommandRunner) {},
			expectedResult: func(result *MergeResult) error {
				if result.BaselineCoverage != 0 {
					return fmt.Errorf("expected baseline coverage 0, got %d", result.BaselineCoverage)
				}
				if result.InputsAnalyzed != 0 {
					return fmt.Errorf("expected 0 inputs analyzed, got %d", result.InputsAnalyzed)
				}
				if result.InputsAdded != 0 {
					return fmt.Errorf("expected 0 inputs added, got %d", result.InputsAdded)
				}
				return nil
			},
		},
		{
			name: "single new input increases coverage",
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/dest")
				fs.AddDir("/src")
				fs.AddFile("/src/input1", []byte("test input"), 10)
				fs.AddDir("/pkg")
			},
			setupRunner: func(runner *MockCommandRunner) {
				// Second call returns increased coverage
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=1x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte("DEBUG finished processing ... initial coverage bits: 150"), nil)
			},
			expectedResult: func(result *MergeResult) error {
				if result.BaselineCoverage != 0 {
					return fmt.Errorf("expected baseline coverage 0, got %d", result.BaselineCoverage)
				}
				if result.FinalCoverage != 150 {
					return fmt.Errorf("expected final coverage 150, got %d", result.FinalCoverage)
				}
				if result.InputsAnalyzed != 1 {
					return fmt.Errorf("expected 1 input analyzed, got %d", result.InputsAnalyzed)
				}
				if result.InputsAdded != 1 {
					return fmt.Errorf("expected 1 input added, got %d", result.InputsAdded)
				}
				if len(result.Inputs) != 1 {
					return fmt.Errorf("expected 1 input in results, got %d", len(result.Inputs))
				}
				if !result.Inputs[0].Added {
					return fmt.Errorf("expected input to be added")
				}
				if result.Inputs[0].CoverageIncrease != 150 {
					return fmt.Errorf("expected coverage increase of 150, got %d", result.Inputs[0].CoverageIncrease)
				}
				return nil
			},
		},
		{
			name: "input already exists in destination",
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/dest")
				fs.AddFile("/dest/input1", []byte("existing"), 8)
				fs.AddDir("/src")
				fs.AddFile("/src/input1", []byte("test input"), 10)
				fs.AddDir("/pkg")
			},
			setupRunner: func(runner *MockCommandRunner) {
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=1x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte("DEBUG finished processing ... initial coverage bits: 100"), nil)
			},
			expectedResult: func(result *MergeResult) error {
				if result.InputsAdded != 0 {
					return fmt.Errorf("expected 0 inputs added, got %d", result.InputsAdded)
				}
				if len(result.Inputs) != 1 {
					return fmt.Errorf("expected 1 input in results, got %d", len(result.Inputs))
				}
				if result.Inputs[0].Added {
					return fmt.Errorf("expected input not to be added")
				}
				if result.Inputs[0].SkippedReason != "already exists" {
					return fmt.Errorf("expected skip reason 'already exists', got %s", result.Inputs[0].SkippedReason)
				}
				return nil
			},
		},
		{
			name: "input does not increase coverage",
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/dest")
				fs.AddDir("/src")
				fs.AddFile("/src/input1", []byte("test input"), 10)
				fs.AddDir("/pkg")
			},
			setupRunner: func(runner *MockCommandRunner) {
				// Only one call since dest is empty (no baseline)
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=1x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte("DEBUG finished processing ... initial coverage bits: 0"), nil)
			},
			expectedResult: func(result *MergeResult) error {
				if result.InputsAdded != 0 {
					return fmt.Errorf("expected 0 inputs added, got %d", result.InputsAdded)
				}
				if result.Inputs[0].SkippedReason != "no coverage increase" {
					return fmt.Errorf("expected skip reason 'no coverage increase', got %s", result.Inputs[0].SkippedReason)
				}
				return nil
			},
		},
		{
			name: "multiple inputs sorted by size",
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/dest")
				fs.AddDir("/src")
				// Add files in non-size order
				fs.AddFile("/src/large", []byte("large input file"), 20)
				fs.AddFile("/src/small", []byte("small"), 5)
				fs.AddFile("/src/medium", []byte("medium file"), 10)
				fs.AddDir("/pkg")
			},
			setupRunner: func(runner *MockCommandRunner) {
				// Baseline
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=0x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte("DEBUG finished processing ... initial coverage bits: 100"), nil)
				// Small file increases coverage
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=1x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte("DEBUG finished processing ... initial coverage bits: 110"), nil)
				// Medium file increases coverage
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=2x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte("DEBUG finished processing ... initial coverage bits: 120"), nil)
				// Large file doesn't increase coverage
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=2x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte("DEBUG finished processing ... initial coverage bits: 120"), nil)
			},
			expectedResult: func(result *MergeResult) error {
				if result.InputsAnalyzed != 3 {
					return fmt.Errorf("expected 3 inputs analyzed, got %d", result.InputsAnalyzed)
				}
				if result.InputsAdded != 2 {
					return fmt.Errorf("expected 2 inputs added, got %d", result.InputsAdded)
				}
				// Check that inputs were processed in size order
				if result.Inputs[0].Name != "small" {
					return fmt.Errorf("expected first input to be 'small', got %s", result.Inputs[0].Name)
				}
				if result.Inputs[1].Name != "medium" {
					return fmt.Errorf("expected second input to be 'medium', got %s", result.Inputs[1].Name)
				}
				if result.Inputs[2].Name != "large" {
					return fmt.Errorf("expected third input to be 'large', got %s", result.Inputs[2].Name)
				}
				return nil
			},
		},
		{
			name: "backup directory already exists",
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/dest")
				fs.AddDir("/src")
				fs.AddDir("/pkg")
				fs.AddDir("/pkg/testdata/fuzz/FuzzTest.bak")
			},
			setupRunner: func(runner *MockCommandRunner) {},
			expectError:   true,
			errorContains: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewMockFileSystem()
			runner := NewMockCommandRunner()
			
			// Setup filesystem
			tt.setupFS(fs)
			
			// Setup command runner
			tt.setupRunner(runner)
			
			cfg := CorpusMergeConfig{
				DestDir:    "/dest",
				SrcDir:     "/src",
				PackageDir: "/pkg",
				FuzzTarget: "FuzzTest",
				CacheDir:   "/tmp/fuzz-merge-1",
				FS:         fs,
				CmdRunner:  runner,
			}
			
			merger := NewCorpusMerger(cfg)
			result, err := merger.Analyze()
			
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tt.errorContains)
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Fatalf("expected error containing '%s', got: %v", tt.errorContains, err)
				}
				return
			}
			
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if tt.expectedResult != nil {
				if err := tt.expectedResult(result); err != nil {
					t.Errorf("result validation failed: %v", err)
				}
			}
		})
	}
}

func TestCorpusMergerApply(t *testing.T) {
	fs := NewMockFileSystem()
	runner := NewMockCommandRunner()
	
	// Setup filesystem
	fs.AddDir("/dest")
	fs.AddDir("/src")
	fs.AddFile("/src/input1", []byte("content1"), 8)
	fs.AddFile("/src/input2", []byte("content2"), 8)
	
	cfg := CorpusMergeConfig{
		DestDir:    "/dest",
		SrcDir:     "/src",
		PackageDir: "/pkg",
		FuzzTarget: "FuzzTest",
		FS:         fs,
		CmdRunner:  runner,
	}
	
	merger := NewCorpusMerger(cfg)
	
	// Create a mock result
	result := &MergeResult{
		InputsAdded: 2,
		Inputs: []MergeInput{
			{Name: "input1", Added: true},
			{Name: "input2", Added: true},
			{Name: "input3", Added: false}, // Not added
		},
	}
	
	// Apply the result
	err := merger.Apply(result)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	
	// Verify files were copied
	if !fs.FileExists("/dest/input1") {
		t.Error("Expected /dest/input1 to exist")
	}
	if !fs.FileExists("/dest/input2") {
		t.Error("Expected /dest/input2 to exist")
	}
	if fs.FileExists("/dest/input3") {
		t.Error("Expected /dest/input3 not to exist")
	}
	
	// Verify content
	content1 := fs.GetFileContent("/dest/input1")
	if string(content1) != "content1" {
		t.Errorf("Expected content1, got %s", string(content1))
	}
}

func TestCorpusMergerProgressReporting(t *testing.T) {
	// Create a mock progress reporter
	var progressCalls []string
	var infoCalls []string
	var warningCalls []string
	
	reporter := &mockProgressReporter{
		onProgress: func(current, total int, message string) {
			progressCalls = append(progressCalls, fmt.Sprintf("%d/%d: %s", current, total, message))
		},
		onInfo: func(message string) {
			infoCalls = append(infoCalls, message)
		},
		onWarning: func(message string) {
			warningCalls = append(warningCalls, message)
		},
	}
	
	fs := NewMockFileSystem()
	runner := NewMockCommandRunner()
	
	// Setup - add existing file to dest to get baseline
	fs.AddDir("/dest")
	fs.AddFile("/dest/existing", []byte("existing"), 8)
	fs.AddDir("/src")
	fs.AddFile("/src/input1", []byte("test"), 4)
	fs.AddDir("/pkg")
	
	// Coverage decreases (to trigger warning)
	runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=1x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
		[]byte("DEBUG finished processing ... initial coverage bits: 100"), nil)
	runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=2x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
		[]byte("DEBUG finished processing ... initial coverage bits: 90"), nil)
	
	cfg := CorpusMergeConfig{
		DestDir:    "/dest",
		SrcDir:     "/src",
		PackageDir: "/pkg",
		FuzzTarget: "FuzzTest",
		CacheDir:   "/tmp/fuzz-merge-1",
		FS:         fs,
		CmdRunner:  runner,
	}
	
	merger := NewCorpusMerger(cfg)
	merger.SetProgressReporter(reporter)
	
	_, err := merger.Analyze()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify progress was reported
	if len(progressCalls) == 0 {
		t.Error("Expected progress calls")
	}
	
	// Verify baseline coverage was reported
	found := false
	for _, call := range infoCalls {
		if strings.Contains(call, "Baseline coverage:") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected baseline coverage info, got info calls: %v", infoCalls)
	}
	
	// Verify warning was issued
	found = false
	for _, call := range warningCalls {
		if strings.Contains(call, "Nondeterministic") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected nondeterministic warning, got warning calls: %v", warningCalls)
	}
}

// mockProgressReporter for testing
type mockProgressReporter struct {
	onProgress func(int, int, string)
	onInfo     func(string)
	onWarning  func(string)
	onError    func(string)
}

func (m *mockProgressReporter) ReportProgress(current, total int, message string) {
	if m.onProgress != nil {
		m.onProgress(current, total, message)
	}
}

func (m *mockProgressReporter) ReportInfo(message string) {
	if m.onInfo != nil {
		m.onInfo(message)
	}
}

func (m *mockProgressReporter) ReportWarning(message string) {
	if m.onWarning != nil {
		m.onWarning(message)
	}
}

func (m *mockProgressReporter) ReportError(message string) {
	if m.onError != nil {
		m.onError(message)
	}
}

// Property-based test for corpus merging
func TestCorpusMergerProperties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random number of files
		numFiles := rapid.IntRange(0, 20).Draw(t, "numFiles")
		
		fs := NewMockFileSystem()
		runner := NewMockCommandRunner()
		
		// Setup directories
		fs.AddDir("/dest")
		fs.AddDir("/src")
		fs.AddDir("/pkg")
		
		// Generate files with random sizes
		var expectedFiles []string
		baseCoverage := 100
		currentCoverage := baseCoverage
		
		for i := 0; i < numFiles; i++ {
			name := fmt.Sprintf("file%d", i)
			size := rapid.Int64Range(1, 1000).Draw(t, fmt.Sprintf("size%d", i))
			fs.AddFile(fmt.Sprintf("/src/%s", name), []byte(strings.Repeat("x", int(size))), size)
			expectedFiles = append(expectedFiles, name)
			
			// Randomly decide if this file increases coverage
			increasesCoverage := rapid.Bool().Draw(t, fmt.Sprintf("increases%d", i))
			if increasesCoverage {
				increase := rapid.IntRange(1, 50).Draw(t, fmt.Sprintf("increase%d", i))
				newCoverage := currentCoverage + increase
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", 
					fmt.Sprintf("-fuzztime=%dx", i+1), "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte(fmt.Sprintf("DEBUG finished processing ... initial coverage bits: %d", newCoverage)), nil)
				currentCoverage = newCoverage
			} else {
				runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", 
					fmt.Sprintf("-fuzztime=%dx", i+1), "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
					[]byte(fmt.Sprintf("DEBUG finished processing ... initial coverage bits: %d", currentCoverage)), nil)
			}
		}
		
		// Set baseline coverage
		runner.SetOutput("go", []string{"test", "-run=^FuzzTest$", "-fuzz=^FuzzTest$", "-fuzztime=0x", "-test.fuzzcachedir=/tmp/fuzz-merge-1"}, 
			[]byte(fmt.Sprintf("DEBUG finished processing ... initial coverage bits: %d", baseCoverage)), nil)
		
		cfg := CorpusMergeConfig{
			DestDir:    "/dest",
			SrcDir:     "/src",
			PackageDir: "/pkg",
			FuzzTarget: "FuzzTest",
			CacheDir:   "/tmp/fuzz-merge-1",
			FS:         fs,
			CmdRunner:  runner,
		}
		
		merger := NewCorpusMerger(cfg)
		result, err := merger.Analyze()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		
		// Properties to verify:
		
		// 1. Number of inputs analyzed should match number of files
		if result.InputsAnalyzed != numFiles {
			t.Errorf("Expected %d inputs analyzed, got %d", numFiles, result.InputsAnalyzed)
		}
		
		// 2. Final coverage should be >= baseline coverage
		if result.FinalCoverage < result.BaselineCoverage {
			t.Errorf("Final coverage %d should be >= baseline %d", result.FinalCoverage, result.BaselineCoverage)
		}
		
		// 3. Number of inputs in result should match analyzed
		if len(result.Inputs) != numFiles {
			t.Errorf("Expected %d inputs in result, got %d", numFiles, len(result.Inputs))
		}
		
		// 4. Inputs marked as added should have positive coverage increase
		for _, input := range result.Inputs {
			if input.Added && input.CoverageIncrease <= 0 {
				t.Errorf("Added input %s should have positive coverage increase, got %d", 
					input.Name, input.CoverageIncrease)
			}
			if !input.Added && input.CoverageIncrease > 0 {
				t.Errorf("Non-added input %s should not have positive coverage increase, got %d", 
					input.Name, input.CoverageIncrease)
			}
		}
		
		// 5. Apply should only copy added inputs
		err = merger.Apply(result)
		if err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
		
		copiedCount := 0
		for _, input := range result.Inputs {
			if fs.FileExists(fmt.Sprintf("/dest/%s", input.Name)) {
				copiedCount++
				if !input.Added {
					t.Errorf("Input %s was copied but not marked as added", input.Name)
				}
			}
		}
		
		if copiedCount != result.InputsAdded {
			t.Errorf("Expected %d files copied, got %d", result.InputsAdded, copiedCount)
		}
	})
}