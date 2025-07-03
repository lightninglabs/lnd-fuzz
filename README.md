# LND Fuzz

Fuzzing seeds for the [Lightning Network
Daemon](https://github.com/lightningnetwork/lnd).

## Tools

This repository includes fuzzing utilities written in Go. See [README_GO_TOOLS.md](README_GO_TOOLS.md) for detailed documentation.

Available tools:
- `corpus-merge`: Merges new fuzzing inputs into an existing corpus
- `cov-profiles`: Collects and combines coverage profiles from multiple fuzz tests
- `profile-diffs`: Compares coverage profiles to identify newly covered code

### Building Tools

```shell
# Build all tools using make
make build

# Or build individually
go build -o bin/corpus-merge ./cmd/corpus-merge
go build -o bin/cov-profiles ./cmd/cov-profiles
go build -o bin/profile-diffs ./cmd/profile-diffs

# Or use go run directly
go run ./cmd/corpus-merge [args...]

# Run tests
make test
```

## Contributing

If you find coverage-increasing inputs while fuzzing LND, please create a pull
request adding them to this repository. Use the `corpus-merge` tool to only
add inputs that increase coverage.

### Example

Here's an example workflow to contribute new inputs for the lnwire
`FuzzAcceptChannel` target. We'll assume the `lnd` and `lnd-fuzz` repositories
are checked out in the current directory.

First create a directory for corpus inputs to be saved in. Use the `lnd-fuzz`
corpus to seed the new corpus, and begin fuzzing:

```shell
export ROOT=$(pwd)
mkdir lnwire_corpus
cp -r lnd-fuzz/lnwire/testdata/fuzz/FuzzAcceptChannel lnwire_corpus/
cd lnd/lnwire
go test -fuzz=FuzzAcceptChannel -parallel=4 -test.fuzzcachedir="$ROOT/lnwire_corpus"
```

After some time, the fuzzer may find some potential coverage-increasing inputs
and save them to `lnwire_corpus/FuzzAcceptChannel/`. We can then merge them into
the `lnd-fuzz` corpus:

```shell
cd $ROOT

# Build the corpus-merge tool (only needed once)
cd lnd-fuzz && go build -o bin/corpus-merge ./cmd/corpus-merge && cd ..

# Merge the new inputs
lnd-fuzz/bin/corpus-merge lnd-fuzz/lnwire/testdata/fuzz/FuzzAcceptChannel \
    lnwire_corpus/FuzzAcceptChannel lnd/lnwire FuzzAcceptChannel
```

Any inputs in `lnwire_corpus/FuzzAcceptChannel` that increase coverage over the
existing `lnd-fuzz` corpus will be copied over. If new inputs were added, create
a pull request to improve the upstream seed corpus:

```shell
cd lnd-fuzz
git add lnwire/testdata/fuzz/FuzzAcceptChannel/*
git commit
```

## Coverage Analysis

To analyze fuzzing coverage across multiple packages:

```shell
# Build the coverage tool
go build -o bin/cov-profiles ./cmd/cov-profiles

# Collect coverage profiles (assumes lnd is in ../lnd)
./bin/cov-profiles ../lnd

# View coverage in HTML
cd ../lnd && go tool cover -html ../lnd-fuzz/coverage/profile
```

To compare coverage between runs:

```shell
# Save current coverage profile
cp coverage/profile coverage/profile.old

# Run fuzzing and collect new coverage
./bin/cov-profiles ../lnd

# Compare profiles
go build -o bin/profile-diffs ./cmd/profile-diffs
./bin/profile-diffs coverage/profile.old coverage/profile coverage/new_blocks.txt

# View newly covered code blocks
cat coverage/new_blocks.txt
```
