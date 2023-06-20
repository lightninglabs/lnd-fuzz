#!/bin/bash -eu
#
# Merges new fuzzing inputs into an existing corpus, only keeping the inputs
# that increase coverage as measured by Go's native fuzzing engine. Prefers
# smaller inputs over larger ones.
#
# Ideally, Go's native fuzzing engine would provide merge functionality via a
# command-line flag. But until https://github.com/golang/go/issues/49290 is
# resolved, we'll have to use our own merging algorithm.
#
# Example usage to merge new inputs into the FuzzPong corpus:
#   $ ./corpus_merge.sh lnwire/testdata/fuzz/FuzzPong \
#       $(go env GOCACHE)/fuzz/github.com/lightningnetwork/lnd/lnwire/FuzzPong \
#       ../lnd/lnwire FuzzPong

if [[ "$#" -ne 4 ]]; then
	echo "Usage: $0 DEST_CORPUS SRC_CORPUS PACKAGE_DIR FUZZ_TARGET_NAME"
	exit 1
fi

readonly DEST_DIR="$1"
readonly SRC_DIR="$2"
readonly PACKAGE_DIR="$3"
readonly FUZZ_TARGET="$4"

readonly FUZZ_TESTDATA_DIR="${PACKAGE_DIR}/testdata/fuzz/${FUZZ_TARGET}"
readonly CACHE_DIR=$(mktemp -d)

function validate_args {
	if [[ -d "${FUZZ_TESTDATA_DIR}.bak" ]]; then
		echo "Error: ${FUZZ_TESTDATA_DIR}.bak already exists"
		exit 1
	fi
	if [[ "${FUZZ_TESTDATA_DIR}" -ef "${SRC_DIR}" ]]; then
		echo "Error: SRC_DIR must not be the testdata fuzz seed" \
			"directory"
		exit 1
	fi
	if [[ "${FUZZ_TESTDATA_DIR}" -ef "${DEST_DIR}" ]]; then
		echo "Error: DEST_DIR must not be the testdata fuzz seed" \
			"directory"
		exit 1
	fi
}

function restore_testdata {
	mv "${FUZZ_TESTDATA_DIR}.bak" "${FUZZ_TESTDATA_DIR}"
}

# measure_coverage returns the coverage count obtained by running the fuzz
# target on the inputs in CACHE_DIR/FUZZ_TARGET. We use a number of tricks
# explained inline below.
function measure_coverage {
	cd "${PACKAGE_DIR}"

	# With fuzzdebug=1 set, the Go fuzzing engine prints extra information.
	# In particular, we're interested in the following line that gets
	# printed after all inputs in the fuzzcachedir have been processed:
	#   DEBUG finished processing ... initial coverage bits: XXX
	export GODEBUG="fuzzdebug=1"

	# Extract the number of inputs we're running on. 
	num_inputs=$(ls "${CACHE_DIR}/${FUZZ_TARGET}" | wc -l)

	go test -run="^${FUZZ_TARGET}$" -fuzz="^${FUZZ_TARGET}$" \
		-fuzztime="${num_inputs}x" -test.fuzzcachedir="${CACHE_DIR}" \
		| grep "initial coverage bits:" | grep -oE "[0-9]+$"
	# Arguments explained:
	# 	
	#   -run="^${FUZZ_TARGET}$" -fuzz="^${FUZZ_TARGET}$"
	# Normally when we pass -fuzz, go-test first runs all unit tests before
	# doing any fuzzing. We can skip this by also passing -run.
        #
	#   -fuzztime="${num_inputs}x"
	# By setting the number of iterations to exactly the number of inputs in
	# the fuzzcachedir, we ensure the Go fuzzing engine stops after
	# measuring baseline coverage and doesn't do any fuzzing.
        #
	#   -test.fuzzcachedir="${CACHE_DIR}"
	# We use our own fuzzcachedir to avoid any cross-contamination with the
	# default fuzzcachedir.
	#
	#   | grep "initial coverage bits:" | grep -oE "[0-9]+$"
	# Extract and return the number of coverage bits the Go fuzzing engine
	# prints after processing all fuzzcachedir inputs.
}


validate_args

# Move any existing testdata seed directory to avoid cross-contamination while
# we measure coverage.
if [[ -d "${FUZZ_TESTDATA_DIR}" ]]; then
	mv "${FUZZ_TESTDATA_DIR}" "${FUZZ_TESTDATA_DIR}.bak"

	# Ensure we restore the backup on exit.
	trap restore_testdata EXIT
fi

mkdir "${CACHE_DIR}/${FUZZ_TARGET}"

# Measure baseline coverage for seeds in DEST_DIR.
coverage=0
if [[ -n $(ls "${DEST_DIR}") ]]; then
	cp "${DEST_DIR}"/* "${CACHE_DIR}/${FUZZ_TARGET}/"
	coverage=$(measure_coverage)
fi
echo "Baseline coverage: ${coverage}"

total=$(ls "${SRC_DIR}" | wc -l)
count=0
added_count=0

# Iterate new corpus from smallest to largest input, greedily adding inputs that
# increase coverage. This is based on libFuzzer's merging algorithm.
for f in $(ls -rS "${SRC_DIR}"); do
	count=$((count + 1))

	echo -ne "\033[2K\r"  # Erase line under cursor
	echo -n "Measuring coverage for input ${count}/${total}"

	# Skip if we already have this input in DEST_DIR.
	[[ -f "${DEST_DIR}/${f}" ]] && continue

	# Add f to our cache dir and see if it increases coverage.
	cp "${SRC_DIR}/${f}" "${CACHE_DIR}/${FUZZ_TARGET}/"
	newcoverage=$(measure_coverage)

	if (( newcoverage > coverage )); then
		coverage="${newcoverage}"
		echo
		echo "Input ${f} increased coverage to ${coverage}"
		cp "${SRC_DIR}/${f}" "${DEST_DIR}/"
		added_count=$((added_count + 1))
		continue
	fi

	# It seems fairly common for coverage measurements to vary slightly
	# between runs. Probably there is some nondeterministic code somewhere
	# in one of our dependencies. Print a warning in case the coverage
	# change is unexpectedly large.
	if (( newcoverage < coverage )); then
		echo
		echo "Warning: nondeterministic fuzz target: coverage" \
		       "decreased from ${coverage} to ${newcoverage}"
	fi

	# Speed up future coverage measurements by removing inputs that don't
	# increase coverage.
	rm "${CACHE_DIR}/${FUZZ_TARGET}/${f}"
done

echo
echo "Added ${added_count} new inputs. Final coverage: ${coverage}"
