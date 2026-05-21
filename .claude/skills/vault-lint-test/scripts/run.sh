#!/usr/bin/env bash
# vault-lint-test: run the local verification chain.
# Stops at the first failure. Prints a per-step status table.

set -uo pipefail

FAST=0
START_AT=1

usage() {
    cat <<EOF
Usage: $(basename "$0") [--fast] [--from=N]

Runs the vault verification chain in order:
  1 format    2 lint    3 test-unit    4 test-sim-simple    5 test-sim-nondeterminism

Stops at the first failure and prints a status table.

  --fast      Skip the two simulation steps (4 and 5).
  --from=N    Resume at step N (1..5). Earlier steps are reported SKIPPED.
EOF
}

for arg in "$@"; do
    case "$arg" in
        --fast) FAST=1 ;;
        --from=*) START_AT="${arg#--from=}" ;;
        -h|--help) usage; exit 0 ;;
        *) echo "Unknown argument: $arg" >&2; usage >&2; exit 2 ;;
    esac
done

if ! [[ "$START_AT" =~ ^[1-5]$ ]]; then
    echo "--from must be 1..5, got: $START_AT" >&2
    exit 2
fi

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
    echo "Not in a git repository." >&2
    exit 2
}
cd "$REPO_ROOT"

LOG_DIR="${TMPDIR:-/tmp}/vault-lint-test"
mkdir -p "$LOG_DIR"

declare -A RESULT DETAIL
STEPS=(format lint test-unit test-sim-simple test-sim-nondeterminism)
for s in "${STEPS[@]}"; do RESULT[$s]="PENDING"; DETAIL[$s]=""; done

keeper_tree_hash() {
    find keeper -name '*.go' -type f -print0 2>/dev/null \
        | sort -z \
        | xargs -0 shasum 2>/dev/null \
        | shasum \
        | awk '{print $1}'
}

extract_failure() {
    local target="$1" log="$2"
    case "$target" in
        lint)
            grep -m1 -E '^[^[:space:]]+\.go:[0-9]+:[0-9]+: ' "$log" \
                || tail -n3 "$log" | tr '\n' ' '
            ;;
        test-unit)
            local name
            name=$(grep -m1 -E '^--- FAIL:' "$log" | awk '{print $3}')
            echo "${name:-(unknown — see log)}"
            ;;
        test-sim-simple|test-sim-nondeterminism)
            local seed block
            seed=$(grep -m1 -iE 'seed[[:space:]]*[:=][[:space:]]*[0-9]+' "$log" \
                | grep -oE '[0-9]+' | head -1)
            block=$(grep -m1 -iE '(block[ _]?height|height)[[:space:]]*[:=][[:space:]]*[0-9]+' "$log" \
                | grep -oE '[0-9]+' | head -1)
            echo "seed=${seed:-?}, block=${block:-?}"
            ;;
        *) tail -n3 "$log" | tr '\n' ' ' ;;
    esac
}

run_step() {
    local idx="$1" target="$2"
    local log="$LOG_DIR/$target.log"

    if [[ $idx -lt $START_AT ]]; then
        RESULT[$target]="SKIPPED"
        DETAIL[$target]="--from=$START_AT"
        return 0
    fi
    if [[ $FAST -eq 1 && $idx -ge 4 ]]; then
        RESULT[$target]="SKIPPED"
        DETAIL[$target]="--fast"
        return 0
    fi

    echo "→ make $target  (log: $log)"
    local before=""
    [[ $target == format ]] && before=$(keeper_tree_hash)

    if ! make "$target" >"$log" 2>&1; then
        RESULT[$target]="FAIL"
        DETAIL[$target]=$(extract_failure "$target" "$log")
        return 1
    fi

    case "$target" in
        format)
            local after
            after=$(keeper_tree_hash)
            if [[ -n "$before" && "$before" != "$after" ]]; then
                RESULT[$target]="MODIFIED"
                DETAIL[$target]="files reformatted in keeper/"
            else
                RESULT[$target]="PASS"
                DETAIL[$target]="no changes needed"
            fi
            ;;
        test-unit)
            local cov tests
            cov=$(grep -E '^total:' "$log" | awk '{print $NF}' | tail -1)
            tests=$(grep -cE '^--- PASS:' "$log" 2>/dev/null || echo 0)
            RESULT[$target]="PASS"
            DETAIL[$target]="${tests} tests, coverage ${cov:-?}"
            ;;
        test-sim-simple|test-sim-nondeterminism)
            RESULT[$target]="PASS"
            DETAIL[$target]=""
            ;;
    esac
    return 0
}

overall=0
for i in "${!STEPS[@]}"; do
    idx=$((i + 1))
    if ! run_step "$idx" "${STEPS[$i]}"; then
        overall=1
        for j in $(seq $((i + 1)) $((${#STEPS[@]} - 1))); do
            RESULT[${STEPS[$j]}]="SKIPPED"
            DETAIL[${STEPS[$j]}]="prior step failed"
        done
        break
    fi
done

echo
LABEL_WIDTH=27
for s in "${STEPS[@]}"; do
    detail="${DETAIL[$s]}"
    if [[ -n "$detail" ]]; then
        printf "make %-${LABEL_WIDTH}s : %-9s — %s\n" "$s" "${RESULT[$s]}" "$detail"
    else
        printf "make %-${LABEL_WIDTH}s : %s\n" "$s" "${RESULT[$s]}"
    fi
done

if [[ $overall -ne 0 ]]; then
    echo
    echo "Logs:      $LOG_DIR"
    failed=""
    for s in "${STEPS[@]}"; do
        [[ "${RESULT[$s]}" == "FAIL" ]] && failed="$s" && break
    done
    [[ -n "$failed" ]] && echo "Re-run:    make $failed"
fi

exit $overall
