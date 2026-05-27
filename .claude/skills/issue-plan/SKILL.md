---
name: issue-plan
description: Pull down a GitHub issue by number, read its body and discussion, and propose an implementation plan grounded in the current repo. Invoke when the user asks to plan, scope, or break down work for a specific issue (e.g., `/issue-plan 123`, "plan issue 456", "what would it take to do #789"). Always prompts the user to confirm before any implementation begins.
---

# issue-plan

Turn a GitHub issue into a concrete implementation plan for **this** repo. The skill is read-only — it produces a plan and waits for explicit approval before touching code.

## Inputs

- **Issue number** (required). If the user did not provide one, ask before running anything.
- **Repo override** (optional). Only needed when the current directory is not a clone of the issue's repo. Pass to `gh` as `--repo <owner/repo>`.

## Prerequisites

- `gh` authenticated (`gh auth status` should succeed).
- Run from inside the target git repo, or pass `--repo`.

## Procedure

1. **Fetch the issue.** Use one `gh` call. Request structured JSON so titles, labels, linked PRs, and the full comment thread are available:

   ```bash
   gh issue view <num> --json number,title,state,url,labels,assignees,milestone,body,comments,closedAt
   ```

   If the issue is `CLOSED`, surface that immediately and ask whether the user still wants a plan (it may already be implemented).

2. **Read every comment.** The plan must reflect the latest direction in the thread, not just the original body — decisions often shift in comments. Note who said what and call out conflicts.

3. **Ground the plan in the repo.** Before writing the plan, locate the modules, files, and types the issue refers to. Use `Grep`/`Glob` (or the `Explore` agent for broad searches) to confirm names, paths, and current behavior. **Do not** invent file paths or function names — cite real ones.

   For cosmos-sdk / Provenance work, also invoke `cosmos-sdk-provenance-knowledge` so the plan respects module boundaries (keepers, msg servers, ante handlers, genesis, upgrade handlers).

4. **Write the plan.** Use the format below. Keep it tight — bullets, file paths with line numbers when known, no prose padding.

5. **Ask to proceed.** End by calling `AskUserQuestion` with the options listed under [Confirmation](#confirmation). Do not edit any files until the user picks an option that authorizes work.

## Plan format

```markdown
## Issue #<num> — <title>

**State:** <OPEN|CLOSED> · **Labels:** <…> · **Link:** <url>

### What the issue asks for
<2–4 bullets summarizing the ask, reconciled with the latest comments. Flag any
unresolved questions or conflicting opinions from the thread.>

### Affected areas
- `path/to/file.go` — <why it's touched>
- `x/<module>/keeper/...` — <why>
- `proto/...` — <why, if proto changes are needed>

### Implementation plan
1. <step 1: concrete change, named function / type / file>
2. <step 2…>
3. <…>

### Testing plan
- <unit / integration / sim tests to add or extend, with target file paths>
- <invocations: `make test-unit`, `make test-sim-simple`, etc.>

### Risks & open questions
- <chain-halt risk, migration risk, breaking API, ambiguous spec, missing decision>

### Out of scope
- <things the issue might imply but this plan intentionally defers>
```

## Confirmation

After printing the plan, call `AskUserQuestion` with one question and these options (in this order):

1. **Proceed with this plan** — start implementing step 1.
2. **Revise the plan** — user will describe changes; rewrite and re-prompt.
3. **Just the plan, no implementation** — stop here.

Do not assume approval from silence, a thumbs-up emoji, or a tangential reply. Wait for an explicit choice.

## Hard rules

- **Read-only until approved.** No `Edit`, `Write`, or `Bash` commands that mutate the repo before the user picks "Proceed".
- **No hallucinated references.** Every file path, function name, or type in the plan must exist in the current tree (or be explicitly marked `(new)`).
- **Quote the issue, don't paraphrase decisions.** When the thread contains a specific directive ("do X not Y"), quote it inline so the user can verify you understood.
- **Surface chain-halt risk.** If the plan touches ABCI paths, genesis, or upgrade handlers, invoke `chain-halt-safety` and list the relevant risks in the "Risks" section before asking to proceed.
- **One issue per invocation.** If the user lists multiple issues, ask which one to plan first.
