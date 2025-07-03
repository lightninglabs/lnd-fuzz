package lndfuzz

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestCoverageCollector_Collect(t *testing.T) {
	tests := []struct {
		name           string
		setupFS        func(*MockFileSystem)
		setupRunner    func(*MockCommandRunner)
		cfg            CovProfilesConfig
		expectError    bool
		expectedResult func(*CoverageResult) error
	}{
		{
			name: "successful collection",
			setupFS: func(fs *MockFileSystem) {
				// Setup fuzz directories
				fs.AddDir("/base/lnwire/testdata/fuzz")
				fs.AddDir("/base/lnwire/testdata/fuzz/FuzzTarget1")
				fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget1/input1", []byte("test"), 0)
				fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget1/input2", []byte("test"), 0)

				fs.AddDir("/base/brontide/testdata/fuzz")
				fs.AddDir("/base/brontide/testdata/fuzz/FuzzTarget2")
				fs.AddFile("/base/brontide/testdata/fuzz/FuzzTarget2/input1", []byte("test"), 0)
			},
			setupRunner: func(runner *MockCommandRunner) {
				// Simulate successful fuzzing runs
				runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget1$", "-fuzz=^FuzzTarget1$", "-fuzztime=2x", "-test.gocoverdir=/base/coverage/FuzzTarget1", "-test.fuzzcachedir=/tmp/cache"}, []byte("ok"), nil)
				runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget2$", "-fuzz=^FuzzTarget2$", "-fuzztime=1x", "-test.gocoverdir=/base/coverage/FuzzTarget2", "-test.fuzzcachedir=/tmp/cache"}, []byte("ok"), nil)
				runner.SetOutput("go", []string{"tool", "covdata", "textfmt", "-i=./coverage/FuzzTarget1,./coverage/FuzzTarget2", "-o=-"}, []byte("mode: set\ncoverage data"), nil)
			},
			cfg: CovProfilesConfig{
				LNDDir:   "/lnd",
				BaseDir:  "/base",
				Packages: []string{"lnwire", "brontide"},
			},
			expectedResult: func(result *CoverageResult) error {
				if len(result.Targets) != 2 {
					return fmt.Errorf("expected 2 targets, got %d", len(result.Targets))
				}
				if result.Targets[0].NumInputs != 2 {
					return fmt.Errorf("expected 2 inputs for first target, got %d", result.Targets[0].NumInputs)
				}
				if len(result.CombinedProfile) == 0 {
					return fmt.Errorf("expected non-empty combined profile")
				}
				return nil
			},
		},
		{
			name: "skip missing fuzz directory",
			setupFS: func(fs *MockFileSystem) {
				// Only setup one package with fuzz directory
				fs.AddDir("/base/lnwire/testdata/fuzz")
				fs.AddDir("/base/lnwire/testdata/fuzz/FuzzTarget1")
				fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget1/input1", []byte("test"), 0)
				// brontide package has no fuzz directory
			},
			setupRunner: func(runner *MockCommandRunner) {
				runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget1$", "-fuzz=^FuzzTarget1$", "-fuzztime=1x", "-test.gocoverdir=/base/coverage/FuzzTarget1", "-test.fuzzcachedir=/tmp/cache"}, []byte("ok"), nil)
				runner.SetOutput("go", []string{"tool", "covdata", "textfmt", "-i=./coverage/FuzzTarget1", "-o=-"}, []byte("mode: set\ncoverage data"), nil)
			},
			cfg: CovProfilesConfig{
				LNDDir:   "/lnd",
				BaseDir:  "/base",
				Packages: []string{"lnwire", "brontide"},
			},
			expectedResult: func(result *CoverageResult) error {
				if len(result.Targets) != 1 {
					return fmt.Errorf("expected 1 target, got %d", len(result.Targets))
				}
				if result.Targets[0].Package != "lnwire" {
					return fmt.Errorf("expected lnwire package, got %s", result.Targets[0].Package)
				}
				return nil
			},
		},
		{
			name: "handle fuzz test failure",
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/base/lnwire/testdata/fuzz")
				fs.AddDir("/base/lnwire/testdata/fuzz/FuzzTarget1")
				fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget1/input1", []byte("test"), 0)
			},
			setupRunner: func(runner *MockCommandRunner) {
				// Simulate failure that's not "no tests to run"
				runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget1$", "-fuzz=^FuzzTarget1$", "-fuzztime=1x", "-test.gocoverdir=/base/coverage/FuzzTarget1", "-test.fuzzcachedir=/tmp/cache"}, []byte("FAIL: something went wrong"), fmt.Errorf("exit 1"))
			},
			cfg: CovProfilesConfig{
				LNDDir:   "/lnd",
				BaseDir:  "/base",
				Packages: []string{"lnwire"},
			},
			expectError: true, // Should fail because no coverage data was collected
		},
		{
			name: "no coverage data collected",
			setupFS: func(fs *MockFileSystem) {
				// Package with no fuzz targets
				fs.AddDir("/base/empty/testdata/fuzz")
			},
			setupRunner: func(runner *MockCommandRunner) {},
			cfg: CovProfilesConfig{
				LNDDir:   "/lnd",
				BaseDir:  "/base",
				Packages: []string{"empty"},
			},
			expectError: true,
		},
		{
			name: "combine coverage profiles",
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/base/lnwire/testdata/fuzz")
				fs.AddDir("/base/lnwire/testdata/fuzz/FuzzTarget1")
				fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget1/input1", []byte("test"), 0)

				fs.AddDir("/base/brontide/testdata/fuzz")
				fs.AddDir("/base/brontide/testdata/fuzz/FuzzTarget2")
				fs.AddFile("/base/brontide/testdata/fuzz/FuzzTarget2/input1", []byte("test"), 0)
			},
			setupRunner: func(runner *MockCommandRunner) {
				// Simulate successful fuzzing runs
				runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget1$", "-fuzz=^FuzzTarget1$", "-fuzztime=1x", "-test.gocoverdir=/base/coverage/FuzzTarget1", "-test.fuzzcachedir=/tmp/cache"}, []byte("ok"), nil)
				runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget2$", "-fuzz=^FuzzTarget2$", "-fuzztime=1x", "-test.gocoverdir=/base/coverage/FuzzTarget2", "-test.fuzzcachedir=/tmp/cache"}, []byte("ok"), nil)
				// Simulate covdata combining profiles
				combinedProfile := `mode: set
github.com/lightningnetwork/lnd/lnwire/file.go:1.1,2.2 1 5
github.com/lightningnetwork/lnd/brontide/file.go:3.3,4.4 1 10`
				runner.SetOutput("go", []string{"tool", "covdata", "textfmt", "-i=./coverage/FuzzTarget1,./coverage/FuzzTarget2", "-o=-"}, []byte(combinedProfile), nil)
			},
			cfg: CovProfilesConfig{
				LNDDir:   "/lnd",
				BaseDir:  "/base",
				Packages: []string{"lnwire", "brontide"},
			},
			expectedResult: func(result *CoverageResult) error {
				profile := string(result.CombinedProfile)
				if !strings.Contains(profile, "mode: set") {
					return fmt.Errorf("expected mode header in profile")
				}
				if !strings.Contains(profile, "lnwire/file.go") {
					return fmt.Errorf("expected lnwire coverage in profile")
				}
				if !strings.Contains(profile, "brontide/file.go") {
					return fmt.Errorf("expected brontide coverage in profile")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock filesystem and command runner
			fs := NewMockFileSystem()
			runner := NewMockCommandRunner()

			// Setup test environment
			if tt.setupFS != nil {
				tt.setupFS(fs)
			}
			if tt.setupRunner != nil {
				tt.setupRunner(runner)
			}

			// Configure with mocks
			cfg := tt.cfg
			cfg.FS = fs
			cfg.CmdRunner = runner
			cfg.CacheDir = "/tmp/cache"

			// Create collector
			collector := NewCoverageCollector(cfg)

			// Collect coverage
			result, err := collector.Collect()

			// Check error
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate result
			if tt.expectedResult != nil {
				if err := tt.expectedResult(result); err != nil {
					t.Errorf("result validation failed: %v", err)
				}
			}

			// Verify timing
			if result.StartTime.After(result.EndTime) {
				t.Errorf("start time after end time")
			}
		})
	}
}

func TestCoverageCollector_Write(t *testing.T) {
	tests := []struct {
		name        string
		result      *CoverageResult
		setupFS     func(*MockFileSystem)
		expectError bool
		verify      func(*MockFileSystem) error
	}{
		{
			name: "successful write",
			result: &CoverageResult{
				CombinedProfile: []byte("mode: set\nprofile data"),
				ProfilePath:     "/base/coverage/profile",
			},
			setupFS: func(fs *MockFileSystem) {
				fs.AddDir("/base")
			},
			verify: func(fs *MockFileSystem) error {
				content := fs.GetFileContent("/base/coverage/profile")
				if content == nil {
					return fmt.Errorf("profile file not found")
				}
				if string(content) != "mode: set\nprofile data" {
					return fmt.Errorf("unexpected profile content: %s", content)
				}
				return nil
			},
		},
		{
			name:        "nil result",
			result:      nil,
			expectError: true,
		},
		{
			name: "empty profile data",
			result: &CoverageResult{
				CombinedProfile: []byte{},
				ProfilePath:     "/base/coverage/profile",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewMockFileSystem()
			if tt.setupFS != nil {
				tt.setupFS(fs)
			}

			cfg := CovProfilesConfig{
				FS: fs,
			}
			collector := NewCoverageCollector(cfg)

			err := collector.Write(tt.result)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.verify != nil {
				if err := tt.verify(fs); err != nil {
					t.Errorf("verification failed: %v", err)
				}
			}
		})
	}
}

func TestCoverageCollector_ProgressReporting(t *testing.T) {
	var progressCalls []string
	var infoCalls []string
	var errorCalls []string

	reporter := &testProgressReporter{
		onProgress: func(current, total int, message string) {
			progressCalls = append(progressCalls, fmt.Sprintf("%d/%d: %s", current, total, message))
		},
		onInfo: func(message string) {
			infoCalls = append(infoCalls, message)
		},
		onError: func(message string) {
			errorCalls = append(errorCalls, message)
		},
	}

	fs := NewMockFileSystem()
	fs.AddDir("/base/lnwire/testdata/fuzz")
	fs.AddDir("/base/lnwire/testdata/fuzz/FuzzTarget1")
	fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget1/input1", []byte("test"), 0)

	runner := NewMockCommandRunner()
	runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget1$", "-fuzz=^FuzzTarget1$", "-fuzztime=1x", "-test.gocoverdir=/base/coverage/FuzzTarget1", "-test.fuzzcachedir=/tmp/cache"}, []byte("ok"), nil)
	runner.SetOutput("go", []string{"tool", "covdata", "textfmt", "-i=./coverage/FuzzTarget1", "-o=-"}, []byte("mode: set"), nil)

	cfg := CovProfilesConfig{
		LNDDir:    "/lnd",
		BaseDir:   "/base",
		Packages:  []string{"lnwire", "missing"},
		FS:        fs,
		CmdRunner: runner,
		CacheDir:  "/tmp/cache",
	}

	collector := NewCoverageCollector(cfg)
	collector.SetProgressReporter(reporter)

	_, err := collector.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify progress reporting
	if len(progressCalls) == 0 {
		t.Errorf("expected progress calls but got none")
	}

	// Should report skipping missing package
	foundSkip := false
	for _, info := range infoCalls {
		if strings.Contains(info, "Skipping missing") {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Errorf("expected skip message for missing package")
	}

	// Should report combining profiles
	foundCombine := false
	for _, info := range infoCalls {
		if strings.Contains(info, "Combining coverage profiles") {
			foundCombine = true
			break
		}
	}
	if !foundCombine {
		t.Errorf("expected combine message")
	}
}

func TestCoverageCollector_CollectTargetCoverage(t *testing.T) {
	fs := NewMockFileSystem()
	fs.AddDir("/base")
	fs.AddDir("/base/lnwire")
	fs.AddDir("/base/lnwire/testdata")
	fs.AddDir("/base/lnwire/testdata/fuzz")
	fs.AddDir("/base/lnwire/testdata/fuzz/FuzzTarget")
	fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget/input1", []byte("data1"), 0)
	fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget/input2", []byte("data2"), 0)
	fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget/input3", []byte("data3"), 0)

	runner := NewMockCommandRunner()
	runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget$", "-fuzz=^FuzzTarget$", "-fuzztime=3x", "-test.gocoverdir=/base/coverage/FuzzTarget", "-test.fuzzcachedir=/cache"}, []byte("coverage collected"), nil)

	cfg := CovProfilesConfig{
		LNDDir:    "/lnd",
		BaseDir:   "/base",
		FS:        fs,
		CmdRunner: runner,
	}

	collector := NewCoverageCollector(cfg)

	coverageDir, numInputs, err := collector.collectTargetCoverage("lnwire", "FuzzTarget", "/cache")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if numInputs != 3 {
		t.Errorf("expected 3 inputs, got %d", numInputs)
	}

	if coverageDir != "coverage/FuzzTarget" {
		t.Errorf("unexpected coverage dir: %s", coverageDir)
	}

	// Verify inputs were copied to cache
	for i := 1; i <= 3; i++ {
		path := fmt.Sprintf("/cache/FuzzTarget/input%d", i)
		if !fs.FileExists(path) {
			t.Errorf("expected file %s to exist in cache", path)
		}
	}

	// Verify coverage directory was created (check both with and without leading slash)
	if !fs.DirExists("/base/coverage/FuzzTarget") && !fs.DirExists("base/coverage/FuzzTarget") {
		t.Errorf("expected coverage directory to be created")
	}

	// Verify command was run with correct arguments
	cmds := runner.GetCommands()
	if len(cmds) == 0 {
		t.Errorf("expected at least one command to be run")
	} else {
		lastCmd := fmt.Sprintf("%s %s", cmds[len(cmds)-1].Name, strings.Join(cmds[len(cmds)-1].Args, " "))
		if !strings.Contains(lastCmd, "-fuzztime=3x") {
			t.Errorf("expected fuzztime=3x in command: %s", lastCmd)
		}
		if !strings.Contains(lastCmd, "-test.gocoverdir=/base/coverage/FuzzTarget") {
			t.Errorf("expected gocoverdir in command: %s", lastCmd)
		}
	}
}

// Property-based testing for CoverageCollector
func TestCoverageCollectorProperties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random package structure
		numPackages := rapid.IntRange(1, 5).Draw(t, "numPackages")
		packages := make([]string, numPackages)

		fs := NewMockFileSystem()
		runner := NewMockCommandRunner()

		expectedTargets := 0
		expectedTotalInputs := 0

		for i := 0; i < numPackages; i++ {
			pkgName := fmt.Sprintf("pkg%d", i)
			packages[i] = pkgName

			// Randomly decide if package has fuzz directory
			hasFuzz := rapid.Bool().Draw(t, "hasFuzz")
			if !hasFuzz {
				continue
			}

			fuzzDir := fmt.Sprintf("/base/%s/testdata/fuzz", pkgName)
			fs.AddDir(fuzzDir)

			// Generate random number of fuzz targets
			numTargets := rapid.IntRange(0, 3).Draw(t, "numTargets")
			for j := 0; j < numTargets; j++ {
				targetName := fmt.Sprintf("FuzzTarget%d", j)
				targetDir := filepath.Join(fuzzDir, targetName)
				fs.AddDir(targetDir)

				// Generate random number of inputs
				numInputs := rapid.IntRange(1, 10).Draw(t, "numInputs")
				for k := 0; k < numInputs; k++ {
					inputFile := filepath.Join(targetDir, fmt.Sprintf("input%d", k))
					fs.AddFile(inputFile, []byte(fmt.Sprintf("data%d", k)), 0)
				}

				expectedTargets++
				expectedTotalInputs += numInputs

				// Setup successful response for each target
				for j := 0; j < numTargets; j++ {
					targetName := fmt.Sprintf("FuzzTarget%d", j)
					// Get the number of inputs for this target
					targetDir := filepath.Join(fuzzDir, targetName)
					entries, _ := fs.ReadDir(targetDir)
					numInputs := len(entries)
					runner.SetOutput("go", []string{"test", "-v", "-cover", fmt.Sprintf("-run=^%s$", targetName), fmt.Sprintf("-fuzz=^%s$", targetName), fmt.Sprintf("-fuzztime=%dx", numInputs), fmt.Sprintf("-test.gocoverdir=/base/coverage/%s", targetName), "-test.fuzzcachedir=/cache"}, []byte("ok"), nil)
				}
			}
		}

		// The mock will provide default output for covdata commands

		cfg := CovProfilesConfig{
			LNDDir:    "/lnd",
			BaseDir:   "/base",
			Packages:  packages,
			FS:        fs,
			CmdRunner: runner,
			CacheDir:  "/cache",
		}

		collector := NewCoverageCollector(cfg)
		result, err := collector.Collect()

		if expectedTargets == 0 {
			// Should error if no targets found
			if err == nil {
				t.Errorf("expected error when no targets found")
			}
			return
		}

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Property 1: Number of targets should match expected
		actualTargets := 0
		for _, target := range result.Targets {
			if target.Error == nil {
				actualTargets++
			}
		}
		if actualTargets != expectedTargets {
			t.Errorf("expected %d targets, got %d", expectedTargets, actualTargets)
		}

		// Property 2: Total inputs should match expected
		actualInputs := 0
		for _, target := range result.Targets {
			if target.Error == nil {
				actualInputs += target.NumInputs
			}
		}
		if actualInputs != expectedTotalInputs {
			t.Errorf("expected %d total inputs, got %d", expectedTotalInputs, actualInputs)
		}

		// Property 3: ProfilePath should be set correctly
		expectedPath := filepath.Join("/base", "coverage", "profile")
		if result.ProfilePath != expectedPath {
			t.Errorf("expected profile path %s, got %s", expectedPath, result.ProfilePath)
		}

		// Property 4: Combined profile should contain data
		if len(result.CombinedProfile) == 0 {
			t.Errorf("expected non-empty combined profile")
		}
	})
}

// testProgressReporter is a test implementation of ProgressReporter.
type testProgressReporter struct {
	onProgress func(current, total int, message string)
	onInfo     func(message string)
	onWarning  func(message string)
	onError    func(message string)
}

func (r *testProgressReporter) ReportProgress(current, total int, message string) {
	if r.onProgress != nil {
		r.onProgress(current, total, message)
	}
}

func (r *testProgressReporter) ReportInfo(message string) {
	if r.onInfo != nil {
		r.onInfo(message)
	}
}

func (r *testProgressReporter) ReportWarning(message string) {
	if r.onWarning != nil {
		r.onWarning(message)
	}
}

func (r *testProgressReporter) ReportError(message string) {
	if r.onError != nil {
		r.onError(message)
	}
}

// TestCoverageCollector_CopyFile tests the copyFile method.
func TestCoverageCollector_CopyFile(t *testing.T) {
	fs := NewMockFileSystem()
	fs.AddFile("/source/file.txt", []byte("test content"), 0)

	cfg := CovProfilesConfig{
		FS: fs,
	}
	collector := NewCoverageCollector(cfg)

	err := collector.copyFile("/source/file.txt", "/dest/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was copied
	content := fs.GetFileContent("/dest/file.txt")
	if content == nil {
		t.Fatalf("destination file not found")
	}

	if string(content) != "test content" {
		t.Errorf("expected 'test content', got '%s'", content)
	}
}

// TestCoverageCollector_NoTestsToRun tests handling of "no tests to run" error.
func TestCoverageCollector_NoTestsToRun(t *testing.T) {
	fs := NewMockFileSystem()
	fs.AddDir("/base/lnwire/testdata/fuzz/FuzzTarget")
	fs.AddFile("/base/lnwire/testdata/fuzz/FuzzTarget/input1", []byte("test"), 0)

	runner := NewMockCommandRunner()
	// Simulate "no tests to run" error which should be ignored
	runner.SetOutput("go", []string{"test", "-v", "-cover", "-run=^FuzzTarget$", "-fuzz=^FuzzTarget$", "-fuzztime=1x", "-test.gocoverdir=/base/coverage/FuzzTarget", "-test.fuzzcachedir=/cache"}, []byte("testing: warning: no tests to run"), fmt.Errorf("exit 1"))
	runner.SetOutput("go", []string{"tool", "covdata", "textfmt", "-i=./coverage/FuzzTarget", "-o=-"}, []byte("mode: set"), nil)

	cfg := CovProfilesConfig{
		LNDDir:    "/lnd",
		BaseDir:   "/base",
		Packages:  []string{"lnwire"},
		FS:        fs,
		CmdRunner: runner,
		CacheDir:  "/cache",
	}

	collector := NewCoverageCollector(cfg)
	result, err := collector.Collect()

	// Should not error on "no tests to run"
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Target should not have an error
	if len(result.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(result.Targets))
	}
	if result.Targets[0].Error != nil {
		t.Errorf("expected no error for target, got: %v", result.Targets[0].Error)
	}
}

func TestCoverageCollector_DefaultPackages(t *testing.T) {
	defaults := DefaultPackages()

	// Verify we have the expected default packages
	expectedPackages := []string{
		"lnwire",
		"brontide",
		"htlcswitch/hop",
		"tlv",
		"watchtower/wtwire",
		"watchtower/wtclient",
		"zpay32",
	}

	if len(defaults) != len(expectedPackages) {
		t.Errorf("expected %d default packages, got %d", len(expectedPackages), len(defaults))
	}

	for i, pkg := range expectedPackages {
		if i >= len(defaults) || defaults[i] != pkg {
			t.Errorf("expected package %d to be %s", i, pkg)
		}
	}
}
