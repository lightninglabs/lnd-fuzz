# Go Fuzzing Tools

This directory contains Go implementations of fuzzing utilities originally written as shell/Python scripts.

## Tools

### corpus-merge

Merges new fuzzing inputs into an existing corpus, only keeping inputs that increase coverage as measured by Go's native fuzzing engine. Prefers smaller inputs over larger ones.

```bash
go run ./cmd/corpus-merge DEST_CORPUS SRC_CORPUS PACKAGE_DIR FUZZ_TARGET_NAME

# Example:
go run ./cmd/corpus-merge lnwire/testdata/fuzz/FuzzPong \
    $(go env GOCACHE)/fuzz/github.com/lightningnetwork/lnd/lnwire/FuzzPong \
    ../lnd/lnwire FuzzPong
```

### cov-profiles

Fetches coverage data for each fuzz test, combines them, and produces a coverage profile that can be analyzed.

```bash
go run ./cmd/cov-profiles LND_DIR

# With custom packages:
go run ./cmd/cov-profiles -packages "lnwire,brontide" LND_DIR

# View coverage in HTML:
cd LND_DIR && go tool cover -html ../lnd-fuzz/coverage/profile
```

### profile-diffs

Compares two Go coverage profiles and outputs the new code blocks that were hit in the second profile but not the first.

```bash
go run ./cmd/profile-diffs <profile1_path> <profile2_path> <output_path>

# Example:
go run ./cmd/profile-diffs coverage/profile.old coverage/profile.new coverage/new_blocks.txt
```

## Building

```bash
# Build all tools
go build -o bin/corpus-merge ./cmd/corpus-merge
go build -o bin/cov-profiles ./cmd/cov-profiles
go build -o bin/profile-diffs ./cmd/profile-diffs
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## Library Usage

These tools expose their core functionality as a Go library for use in other projects:

```go
import "github.com/lightninglabs/lnd-fuzz"

// Merge corpus
cfg := lndfuzz.CorpusMergeConfig{
    DestDir:    "dest/corpus",
    SrcDir:     "src/corpus",
    PackageDir: "path/to/package",
    FuzzTarget: "FuzzMyTarget",
}
err := lndfuzz.MergeCorpus(cfg)

// Compare coverage profiles
diff, err := lndfuzz.CompareCoverageProfiles("profile1.txt", "profile2.txt")
err = lndfuzz.WriteProfileDiff(diff, "output.txt")

// Collect coverage profiles
cfg := lndfuzz.CovProfilesConfig{
    LNDDir:  "/path/to/lnd",
    BaseDir: "/path/to/lnd-fuzz",
}
err := lndfuzz.CollectCoverageProfiles(cfg)
```

## Dependencies

- Go 1.21 or later
- `pgregory.net/rapid` for property-based testing