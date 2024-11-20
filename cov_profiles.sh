#!/bin/bash
#
# Fetches coverage data for each fuzz test, combines them, and produces a
# coverage profile that can be analyzed.
if [[ "$#" -ne 1 ]]; then
    echo "Usage: $0 LND_DIR"
    exit 1
fi

readonly LND_DIR=$1
readonly CACHE_DIR=$(mktemp -d)

# Get the directory of cov_profiles.sh in case it is being called not in
# lnd-fuzz.
readonly BASE_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

readonly packages=("lnwire" "brontide" "htlcswitch/hop" "tlv" "watchtower/wtwire" "watchtower/wtclient" "zpay32") 

# collect_combine_cov_profiles collects coverage for each fuzzing package and
# then combines them.
function collect_combine_cov_profiles {
    local coverage_dirs=()

    # Collect coverage profiles.
    for p in ${packages[@]}; do
        cd "${BASE_DIR}/${p}/testdata/fuzz"

        for f in $(ls $PWD); do
            # Move corpus to CACHE_DIR.
            mkdir -p "${CACHE_DIR}/${f}"
            cp -a "${BASE_DIR}/${p}/testdata/fuzz/${f}"/ "${CACHE_DIR}/${f}/"
            num_inputs=$(ls "${CACHE_DIR}/${f}/" | wc -l | xargs)
            mkdir -p "${BASE_DIR}/coverage/${f}"

            cd "${LND_DIR}/${p}/"

            go test -v -cover -run="^${f}$" -fuzz="^${f}$" \
                -fuzztime="${num_inputs}x" \
                -test.gocoverdir="${BASE_DIR}/coverage/${f}" \
                -test.fuzzcachedir="${CACHE_DIR}"

            coverage_dirs+=("coverage/${f}")
        done

        cd $BASE_DIR
    done

    # Combine coverage profiles.
    local profile_str=""
    local coverage_dirs_len=${#coverage_dirs[@]}

    for (( i = 1; i <= $coverage_dirs_len; i++ ))
    do
        let "index = $i - 1"
        if [[ $i -eq $coverage_dirs_len ]]
        then
            profile_str+="./${coverage_dirs[$index]}"
            break
        fi

        profile_str+="./${coverage_dirs[$index]},"
    done

    go tool covdata textfmt -i=$profile_str -o coverage/profile
}


collect_combine_cov_profiles
