---
name: coderabbit
description: Summarize CodeRabbit PR review conversations as a compact resolved list (title only, crossed out) and a detailed unresolved section grouped by file, with a colored status indicator at the end. Use when the user asks to see, list, summarize, or triage CodeRabbit feedback on a pull request, or runs `/coderabbit <PR#>`.
version: 0.1.0
---

# CodeRabbit PR Review Summary

Fetches CodeRabbit's review-thread conversations on a pull request and renders:

1. **Resolved** — a compact bullet list of `path — title` lines wrapped in `~~strikethrough~~`. Prose is dropped (already handled).
2. **Unresolved** — full detail (title + prose), grouped under a bold file header.
3. **Status indicator** — a single colored line at the end summarizing the state.

## Install

Drop the `coderabbit/` folder into one of:

- **User-scope** (available everywhere): `~/.claude/skills/coderabbit/`
- **Project-scope** (committed alongside a repo): `<repo>/.claude/skills/coderabbit/`

Restart Claude Code, then invoke with `/coderabbit <PR#>`.

## Prerequisites

- **`gh`** — authenticated via `gh auth login`. The skill uses `gh api graphql`.
- **`jq`** — used to filter and transform the GraphQL response.
- **`git`** — must be run from inside a git repo whose `origin` (or specified remote) points at a GitHub repository.

## Input

A single PR number. If the user did not provide one, ask before running.

## Steps

All scripts live in `${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/skills/coderabbit}/scripts/`. Use that path when invoking them.

1. **Verify a git repo** — run `scripts/check-git-repo.sh`. If it fails, stop and tell the user the current directory isn't a git repository.
2. **Resolve the repo slug** — run `scripts/get-git-remote.sh` to get `owner/repo`.
3. **Fetch CodeRabbit conversations** — run:
   ```sh
   scripts/get-pr-comments.sh <PR#> --repo <owner/repo> \
     | jq '[ .[] | select(.comments | any(.author | test("^coderabbit"; "i"))) ]'
   ```
   The `jq` filter keeps any conversation where at least one comment's author login starts with `coderabbit` (covers `coderabbitai`, `coderabbitai[bot]`, etc.). If the result is empty, also try without the filter and report whatever bot logins appeared so the user can confirm.
4. **Partition** into `isResolved == true` and `isResolved == false`.
5. **Render** the output exactly as specified below. Do not add a preamble, summary, or trailing commentary.

## Output format

For each conversation, parse `.comments[0].body` and extract:

- **Title** — the first `**...**` bolded span in the body (CodeRabbit always leads with one). If the body opens with an italics-wrapped severity line like `_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_`, drop that line.
- **Prose** — everything after the title's blank line. Trim trailing whitespace. CodeRabbit sometimes appends a fenced ` ```suggestion ` block or a `<details>` "🤖 Prompt for AI Agents" block — keep them inline as part of the prose.
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

- `0` unresolved → `🟢 All CodeRabbit feedback resolved.`
- `1`–`3` unresolved → `🟡 <N> CodeRabbit conversation(s) still need attention.`
- `4`+ unresolved → `🔴 <N> CodeRabbit conversations still need attention.`

Do not add any other preamble, summary, or trailing commentary beyond the two sections and this status line.

## Notes

- This skill only covers **review-thread conversations**. CodeRabbit's top-level walkthrough/summary comment (an issue comment) is intentionally excluded because issue comments can't be resolved.
- Strikethrough (`~~...~~`) is GitHub-flavored markdown; it renders as crossed-out text in the Claude Code terminal.
