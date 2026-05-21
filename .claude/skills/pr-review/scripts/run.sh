#!/usr/bin/env bash
# pr-review: fetch all unresolved PR review threads, group by author, render markdown.
# Known bots (CodeRabbit, Greptile) are listed first; other bots, then humans alphabetical.

set -euo pipefail

usage() {
    cat >&2 <<EOF
Usage: $(basename "$0") <pr-number> [--repo <owner/repo>]

Fetches all unresolved review-thread discussions on a PR and groups them by
the thread originator. Output sections are ordered:
  1. CodeRabbit (if present)
  2. Greptile   (if present)
  3. Other bots (alphabetical)
  4. Humans     (alphabetical)
EOF
    exit 1
}

pr=""
repo=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --repo) repo="$2"; shift 2 ;;
        -h|--help) usage ;;
        *)
            if [[ -z "$pr" ]]; then
                pr="$1"; shift
            else
                echo "Unexpected argument: $1" >&2; usage
            fi
            ;;
    esac
done

[[ -z "$pr" ]] && usage

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -z "$repo" ]]; then
    repo="$("$script_dir/get-git-remote.sh")"
fi

threads_json="$("$script_dir/get-pr-comments.sh" "$pr" --unresolved-only --repo "$repo")"

count="$(jq 'length' <<<"$threads_json")"

if [[ "$count" == "0" ]]; then
    echo "🟢 No open PR discussions."
    exit 0
fi

body="$(jq -r '
  def normalize_author:
    if test("^coderabbit"; "i") then "CodeRabbit"
    elif test("^greptile"; "i") then "Greptile"
    else . end;

  def author_priority:
    if . == "CodeRabbit" then 0
    elif . == "Greptile" then 1
    elif endswith("[bot]") then 2
    else 3
    end;

  def strip_preamble:
    # CodeRabbit severity preamble: _⚠️ ..._ | _🟡 ..._
    sub("^_[^_\\n]+_(\\s*\\|\\s*_[^_\\n]+_)*\\s*\\n+"; "")
    # Greptile priority badge HTML (anchor wrapping an img)
    | sub("^<a[^>]*>.*?</a>\\s*\\n+"; ""; "s");

  def extract_title:
    if test("\\*\\*[^*]+\\*\\*") then
      (capture("\\*\\*(?<t>[^*]+)\\*\\*") | .t)
    else
      (split("\n")[0] | .[0:160])
    end;

  def extract_prose:
    if test("\\*\\*[^*]+\\*\\*") then
      sub("^[\\s\\S]*?\\*\\*[^*]+\\*\\*\\s*\\n*"; ""; "s")
    else
      sub("^[^\\n]*\\n*"; "")
    end
    | sub("\\s+$"; "");

  def render_thread:
    . as $t
    | ($t.body | strip_preamble) as $clean
    | ($clean | extract_title) as $title
    | ($clean | extract_prose) as $prose
    | (if $t.line == null then "" else " (line \($t.line))" end) as $lineref
    | "- **\($title)**\($lineref)\n" +
      (if ($prose | length) > 0
         then ($prose | split("\n") | map("  " + .) | join("\n")) + "\n"
         else "" end);

  [.[] | {
      author: (.comments[0].author | normalize_author),
      path: .path,
      line: .line,
      body: .comments[0].body
  }]
  | group_by(.author)
  | map({
      author: .[0].author,
      priority: (.[0].author | author_priority),
      threads: (. | sort_by([.path, (.line // 999999)]))
  })
  | sort_by([.priority, (.author | ascii_downcase)])
  | map(
      "### \(.author) (\(.threads | length) unresolved)\n" +
      (.threads
       | group_by(.path)
       | map(
           "\n**`\(.[0].path)`**\n" +
           (map(render_thread) | join(""))
         )
       | join("")
      )
    )
  | join("\n")
' <<<"$threads_json")"

author_count="$(jq -r '
  [.[] | .comments[0].author
    | (if test("^coderabbit"; "i") then "CodeRabbit"
       elif test("^greptile"; "i") then "Greptile"
       else . end)
  ] | unique | length
' <<<"$threads_json")"

echo "$body"
echo

reviewer_word="reviewer"
[[ "$author_count" != "1" ]] && reviewer_word="reviewers"

if (( count <= 3 )); then
    discussion_word="discussion"
    [[ "$count" != "1" ]] && discussion_word="discussions"
    echo "🟡 $count open $discussion_word across $author_count $reviewer_word."
else
    echo "🔴 $count open discussions across $author_count $reviewer_word."
fi
