#!/usr/bin/env bash
# branch-diff-analysis: scope work to the current branch's diff against a base ref.
# Outputs a structured report: file categories with line counts, source-vs-test
# pairing check, and function-level summary from diff hunk headers.

set -uo pipefail

BASE="main"
for arg in "$@"; do
    case "$arg" in
        --base=*) BASE="${arg#--base=}" ;;
        -h|--help)
            cat <<EOF
Usage: $(basename "$0") [--base=<ref>]

  --base=<ref>   Base ref to diff against. Default: main.
EOF
            exit 0
            ;;
        *) echo "Unknown argument: $arg" >&2; exit 2 ;;
    esac
done

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
    echo "Not in a git repository." >&2
    exit 2
}
cd "$REPO_ROOT"

if ! git rev-parse --verify "$BASE" >/dev/null 2>&1; then
    echo "Base ref not found locally: $BASE" >&2
    echo "Try: git fetch origin $BASE:$BASE" >&2
    exit 2
fi

CURRENT="$(git rev-parse --abbrev-ref HEAD)"
MERGE_BASE="$(git merge-base "$BASE" HEAD)"

FILES="$(git diff "$BASE...HEAD" --name-only)"
if [[ -z "$FILES" ]]; then
    echo "No changes between $BASE and HEAD."
    exit 0
fi

NUMSTAT="$(git diff "$BASE...HEAD" --numstat)"

GO_SRC=()
GO_TEST=()
PROTO=()
GENERATED=()
CONFIG=()
DOCS=()
OTHER=()

while IFS= read -r f; do
    [[ -z "$f" ]] && continue
    case "$f" in
        *.pb.go|*.pulsar.go|*.pb.gw.go|client/docs/statik/statik.go|docs/static/openapi.yml)
            GENERATED+=("$f") ;;
        *_test.go)
            GO_TEST+=("$f") ;;
        *.go)
            GO_SRC+=("$f") ;;
        *.proto)
            PROTO+=("$f") ;;
        Makefile|*.mk|go.mod|go.sum|.gitignore|.golangci.yml|.golangci.yaml|.github/workflows/*|.claude/settings*.json)
            CONFIG+=("$f") ;;
        *.md|spec/*|docs/*)
            DOCS+=("$f") ;;
        *)
            OTHER+=("$f") ;;
    esac
done <<< "$FILES"

file_stats() {
    local path="$1"
    awk -v p="$path" '$3==p {printf "+%s / -%s", $1, $2}' <<<"$NUMSTAT"
}

print_category() {
    local title="$1"; shift
    local count=$#
    [[ $count -eq 0 ]] && return
    echo
    echo "## $title ($count)"
    local f
    for f in "$@"; do
        local stats
        stats=$(file_stats "$f")
        printf "  %-60s  %s\n" "$f" "${stats:-—}"
    done
}

total=$(wc -l <<<"$FILES" | tr -d ' ')

echo "# Branch diff: $CURRENT vs $BASE"
echo "Merge base:    $MERGE_BASE"
echo "Files changed: $total"

print_category "Go source"      "${GO_SRC[@]+"${GO_SRC[@]}"}"
print_category "Go tests"       "${GO_TEST[@]+"${GO_TEST[@]}"}"
print_category "Protobuf"       "${PROTO[@]+"${PROTO[@]}"}"
print_category "Generated"      "${GENERATED[@]+"${GENERATED[@]}"}"
print_category "Config / build" "${CONFIG[@]+"${CONFIG[@]}"}"
print_category "Docs"           "${DOCS[@]+"${DOCS[@]}"}"
print_category "Other"          "${OTHER[@]+"${OTHER[@]}"}"

if [[ ${#GO_SRC[@]} -gt 0 ]]; then
    echo
    echo "## Test coverage cross-check"
    test_dirs=""
    if [[ ${#GO_TEST[@]} -gt 0 ]]; then
        for t in "${GO_TEST[@]}"; do
            test_dirs+="$(dirname "$t")"$'\n'
        done
    fi
    untested=()
    for src in "${GO_SRC[@]}"; do
        dir="$(dirname "$src")"
        if ! grep -qxF "$dir" <<<"$test_dirs"; then
            untested+=("$src")
        fi
    done
    if [[ ${#untested[@]} -eq 0 ]]; then
        echo "  All changed Go source files have a sibling *_test.go modification."
    else
        echo "  Changed Go source files without a matching test change in the same package:"
        for f in "${untested[@]}"; do
            echo "    - $f"
        done
    fi
fi

if [[ ${#GO_SRC[@]} -gt 0 ]]; then
    echo
    echo "## Changed functions (from diff hunk headers)"
    git diff "$BASE...HEAD" --unified=0 -- '*.go' \
        | awk '
            /^diff --git / {
                # extract path: "diff --git a/path b/path"
                file=$3
                sub(/^a\//, "", file)
                next
            }
            /^@@.*@@.*func / {
                # @@ -10,5 +10,7 @@ func (k Keeper) Foo(ctx sdk.Context) error {
                idx = index($0, "func ")
                sig = substr($0, idx)
                sub(/\{.*$/, "", sig)
                sub(/[[:space:]]+$/, "", sig)
                print file " :: " sig
            }
        ' \
        | sort -u
fi
