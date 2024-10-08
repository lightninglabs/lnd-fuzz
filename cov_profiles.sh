#!/bin/bash
#
# Fetches coverage data for each fuzz test, combines them, and produces a
# coverage profile that can be analyzed.

set -e

if [[ "$#" -ne 1 ]]; then
    echo "Usage: $0 LND_DIR"
    exit 1
fi

readonly LND_DIR=$1
readonly CACHE_DIR=$(mktemp -d)

# Get the directory of cov_profiles.sh in case it is being called not in
# lnd-fuzz.
readonly BASE_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

readonly PACKAGES=("lnwire" "brontide" "htlcswitch/hop" "tlv" "watchtower/wtwire" "watchtower/wtclient" "zpay32") 

# collect_combine_cov_profiles collects coverage for each fuzzing package and
# then combines them.
function collect_combine_cov_profiles {
    local coverage_dirs=()

    # Collect coverage profiles.
    for p in ${PACKAGES[@]}; do
        pushd "${BASE_DIR}/${p}/testdata/fuzz"

        for f in $(ls $PWD); do
            # Copy corpus to CACHE_DIR.
            mkdir -p "${CACHE_DIR}/${f}"
            cp -a "${BASE_DIR}/${p}/testdata/fuzz/${f}"/ "${CACHE_DIR}/${f}/"
            num_inputs=$(find "${CACHE_DIR}/${f}/" -maxdepth 1 -type f | wc -l | tr -d '[:space:]')
            mkdir -p "${BASE_DIR}/coverage/${f}"

            pushd "${LND_DIR}/${p}/"

            go test -v -cover -run="^${f}$" -fuzz="^${f}$" \
                -fuzztime="${num_inputs}x" \
                -test.gocoverdir="${BASE_DIR}/coverage/${f}" \
                -test.fuzzcachedir="${CACHE_DIR}"

            coverage_dirs+=("coverage/${f}")

            popd
        done

        popd
    done

    # Combine coverage profiles.
    local profile_str=""
    local coverage_dirs_len=${#coverage_dirs[@]}

    for ((i = 0; i < coverage_dirs_len - 1; i++)); do
    profile_str+="./${coverage_dirs[$i]},"
        done
    profile_str+="./${coverage_dirs[coverage_dirs_len-1]}"

    go tool covdata textfmt -i=$profile_str -o coverage/profile
}


collect_combine_cov_profiles
