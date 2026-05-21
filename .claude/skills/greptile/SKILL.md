---
name: greptile
description: Summarize Greptile PR review conversations as a compact resolved list (title only, crossed out) and a detailed unresolved section grouped by file, with a colored status indicator at the end. Use when the user asks to see, list, summarize, or triage Greptile feedback on a pull request, or runs `/greptile <PR#>`.
version: 0.3.0
---

# Greptile PR Review Summary

Fetches Greptile's review-thread conversations on a pull request and renders:

1. **Resolved** — a compact bullet list of `path — title` lines wrapped in `~~strikethrough~~`. Prose is dropped (already handled).
2. **Unresolved** — full detail (title + prose), grouped under a bold file header.
3. **Status indicator** — a single colored line at the end summarizing the state.

## Install

Drop the `greptile/` folder into one of:

- **User-scope** (available everywhere): `~/.claude/skills/greptile/`
- **Project-scope** (committed alongside a repo): `<repo>/.claude/skills/greptile/`

Restart Claude Code, then invoke with `/greptile <PR#>`.

## Prerequisites

- **`gh`** — authenticated via `gh auth login`. The skill uses `gh api graphql`.
- **`jq`** — used to filter and transform the GraphQL response.
- **`git`** — must be run from inside a git repo whose `origin` (or specified remote) points at a GitHub repository.

## Input

A single PR number. If the user did not provide one, ask before running.

## Steps

All scripts live in `${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/skills/greptile}/scripts/`. Use that path when invoking them.

1. **Verify a git repo** — run `scripts/check-git-repo.sh`. If it fails, stop and tell the user the current directory isn't a git repository.
2. **Resolve the repo slug** — run `scripts/get-git-remote.sh` to get `owner/repo`.
3. **Fetch Greptile conversations** — run:
   ```sh
   scripts/get-pr-comments.sh <PR#> --repo <owner/repo> \
     | jq '[ .[] | select(.comments | any(.author | test("^greptile"; "i"))) ]'
   ```
   The `jq` filter keeps any conversation where at least one comment's author login starts with `greptile` (covers `greptile-apps[bot]`, `greptileai[bot]`, etc.). If the result is empty, also try without the filter and report whatever bot logins appeared so the user can confirm.
4. **Partition** into `isResolved == true` and `isResolved == false`.
5. **Render** the output exactly as specified below. Do not add a preamble, summary, or trailing commentary.

## Output format

For each conversation, parse `.comments[0].body` and extract:

- **Title** — the first `**...**` bolded span in the body (Greptile always leads with one). If the body opens with an `<a ...><img ...></a>` priority badge, drop that HTML.
- **Prose** — everything after the title's blank line. Trim trailing whitespace.
- **Line number** — `.line` (may be `null` for outdated threads; omit the `(line N)` suffix when absent).

### Resolved section

A flat bullet list, one bullet per conversation, sorted alphabetically by path then by line number ascending (nulls last). Prose is dropped — only path + title.

```markdown
### Resolved
- ~~`path/to/file.go` — **<title>**~~
- ~~`other/file.ts` — **<title>**~~
```

### Unresolved section

Group bullets under a bold path header. Within the section, sort files alphabetically; within a file, sort by line number ascending (nulls last).

```markdown
### Unresolved

**`path/to/file.go`**
- **<title>** (line 17)
  <prose>
- **<another title>** (line 42)
  <prose>

**`other/file.ts`**
- **<title>** (line 88)
  <prose>
```

If either section has no conversations, render the heading followed by `_none_` on the next line.

### Status indicator

End the output with a single line based on the count of **unresolved** conversations:

- `0` unresolved → `🟢 All Greptile feedback resolved.`
- `1`–`3` unresolved → `🟡 <N> Greptile conversation(s) still need attention.`
- `4`+ unresolved → `🔴 <N> Greptile conversations still need attention.`

Do not add any other preamble, summary, or trailing commentary beyond the two sections and this status line.

## Notes

- This skill only covers **review-thread conversations**. Greptile's top-level summary comment (an issue comment) is intentionally excluded because issue comments can't be resolved.
- Strikethrough (`~~...~~`) is GitHub-flavored markdown; it renders as crossed-out text in the Claude Code terminal.
