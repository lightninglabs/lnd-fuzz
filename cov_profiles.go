package lndfuzz

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CovProfilesConfig contains configuration for coverage profile collection.
type CovProfilesConfig struct {
	LNDDir   string
	BaseDir  string // lnd-fuzz directory
	CacheDir string
	Packages []string

	// Optional dependencies for testing
	FS        FileSystem
	CmdRunner CommandRunner
}

// DefaultPackages returns the default list of packages to collect coverage for.
func DefaultPackages() []string {
	return []string{
		"lnwire",
		"brontide",
		"htlcswitch/hop",
		"tlv",
		"watchtower/wtwire",
		"watchtower/wtclient",
		"zpay32",
	}
}

// CollectCoverageProfiles is a backward compatibility wrapper for the new CoverageCollector.
// It collects coverage profiles for each fuzzing package
// and combines them into a single profile.
//
// Deprecated: Use NewCoverageCollector for more control and better testability.
func CollectCoverageProfiles(cfg CovProfilesConfig) error {
	collector := NewCoverageCollector(cfg)

	// Use a simple console reporter
	collector.SetProgressReporter(&consoleReporter{})

	// Collect coverage
	result, err := collector.Collect()
	if err != nil {
		return err
	}

	// Write to disk
	if err := collector.Write(result); err != nil {
		return err
	}

	fmt.Printf("Coverage profile written to: %s\n", result.ProfilePath)
	fmt.Println("View coverage in HTML with:")
	fmt.Printf("  cd %s && go tool cover -html ../lnd-fuzz/coverage/profile\n", cfg.LNDDir)

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
