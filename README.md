# LND Fuzz

Fuzzing seeds for the [Lightning Network
Daemon](https://github.com/lightningnetwork/lnd).

## Contributing

If you find coverage-increasing inputs while fuzzing LND, please create a pull
request adding them to this repository. Use the `corpus_merge.sh` script to only
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
lnd-fuzz/corpus_merge.sh lnd-fuzz/lnwire/testdata/fuzz/FuzzAcceptChannel \
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
