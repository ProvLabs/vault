---
name: agent-memory
description: Persistent, file-based agent memory system used by every sub-agent in this repo. Invoke at the start of any agent session to load the rules for writing, reading, organizing, and pruning memories under .claude/agent-memory/<agent-name>/. The calling agent must already know its own memory directory path — pass it in args (e.g., args="path=.claude/agent-memory/go-code-reviewer").
---

# Agent Memory System

You have a persistent, file-based memory system at the path provided in `args` (e.g., `.claude/agent-memory/<your-agent-name>/`). `.claude/agent-memory/` is gitignored, so the directory is local to this clone and may not exist yet — create it on first write (e.g., `mkdir -p` via Bash, or rely on the Write tool to create parent directories) before writing.

Build up this memory system over time so future conversations have a complete picture of who the user is, how they want to collaborate, what behaviors to avoid or repeat, and the context behind the work.

If the user explicitly asks you to remember something, save it immediately as whichever type fits best — **unless** the content falls under "What NOT to save" below. In that case, explain why it's excluded and ask for a non-obvious, durable takeaway to store instead. If they ask you to forget something, find and remove the relevant entry.

## Types of memory

<types>
<type>
  <name>user</name>
  <description>Information about the user's role, goals, responsibilities, and knowledge. Helps tailor future behavior to the user's preferences and perspective. Avoid memories that read as negative judgement or are irrelevant to the work.</description>
  <when_to_save>When you learn details about the user's role, preferences, responsibilities, or knowledge.</when_to_save>
  <how_to_use>When your work should be informed by the user's profile. Frame explanations relative to what they already know.</how_to_use>
</type>
<type>
  <name>feedback</name>
  <description>Guidance the user has given on how to approach work — both what to avoid and what to keep doing. Record from failure AND success: corrections alone leave you overly cautious; capture validated approaches too.</description>
  <when_to_save>When the user corrects your approach ("no, don't", "stop doing X") OR confirms a non-obvious approach worked ("yes, keep doing that", accepts an unusual choice without pushback). Include *why* so you can judge edge cases later.</when_to_save>
  <how_to_use>Let these memories guide behavior so the user doesn't have to repeat guidance.</how_to_use>
  <body_structure>Lead with the rule, then a **Why:** line (the reason the user gave) and a **How to apply:** line (when/where this kicks in).</body_structure>
</type>
<type>
  <name>project</name>
  <description>Information about ongoing work, goals, initiatives, bugs, or incidents that isn't derivable from the code or git history. Helps you understand context and motivation.</description>
  <when_to_save>When you learn who is doing what, why, or by when. Project state changes quickly — keep it current. Always convert relative dates to absolute dates ("Thursday" → "2026-03-05").</when_to_save>
  <how_to_use>Use to make better-informed suggestions about the user's request.</how_to_use>
  <body_structure>Lead with the fact/decision, then a **Why:** line (motivation/constraint) and a **How to apply:** line (how it should shape your suggestions).</body_structure>
</type>
<type>
  <name>reference</name>
  <description>Pointers to where information lives in external systems (Linear projects, Grafana dashboards, Slack channels, etc.).</description>
  <when_to_save>When you learn about external resources and their purpose.</when_to_save>
  <how_to_use>When the user references an external system or information that may live in one.</how_to_use>
</type>
</types>

## What NOT to save

- Code patterns, conventions, architecture, file paths, or project structure — these can be derived by reading the current project state.
- Git history, recent changes, or who-changed-what — `git log` / `git blame` are authoritative.
- Debugging solutions or fix recipes — the fix is in the code; the commit has the context.
- Anything already documented in CLAUDE.md or GEMINI.md.
- Ephemeral task details: in-progress work, current conversation state.

**Exclusion precedence**: these exclusions override an explicit save request. If the user asks you to save a PR list or activity summary, ask what was *surprising* or *non-obvious* — that is the part worth keeping.

## How to save

Two steps:

**Step 1** — write the memory to its own file (e.g., `user_role.md`, `feedback_testing.md`) with this frontmatter:

```markdown
---
name: {{short-kebab-case-slug}}
description: {{one-line summary used to judge relevance later}}
type: {{user | feedback | project | reference}}
---

{{memory content — for feedback/project, structure as: rule/fact, then **Why:** and **How to apply:** lines. Link related memories with [[their-name]].}}
```

In the body, link to related memories with `[[name]]`. A `[[name]]` that doesn't yet match an existing memory is fine — it marks something worth writing later.

**Step 2** — add a pointer to that file in `MEMORY.md`: `- [Title](file.md) — one-line hook`. No frontmatter. Keep each entry under ~150 characters. Lines after 200 will be truncated.

- Keep `name`, `description`, and `type` in sync with the body.
- Organize semantically by topic, not chronologically.
- Update or remove memories that turn out wrong or outdated. Don't write duplicates — update an existing memory if possible.

## When to access memories

- When memories seem relevant, or the user references prior-conversation work.
- Always when the user explicitly asks you to check, recall, or remember.
- If the user says to *ignore* memory: don't apply, cite, compare, or mention memory content.
- Memories can become stale. Before acting on a memory, verify it against current code. If the memory conflicts with what you observe, trust observation and update the memory.

## Before recommending from memory

A memory that names a function, file, or flag claims it existed *when written*. It may have been renamed or removed. Before recommending:

- Memory names a file path → check the file exists.
- Memory names a function or flag → grep for it.
- User is about to act on the recommendation → verify first.

A memory that summarizes repo state is frozen in time. If the user asks about *recent* or *current* state, prefer `git log` and reading code over recalling the snapshot.

## Memory vs other persistence

Memory is for information useful in *future* conversations. For the current conversation, prefer:
- **Plan** when aligning with the user on a non-trivial implementation approach.
- **Tasks** when tracking discrete steps in this session.

This memory directory is gitignored and local to each developer's clone — memories are not shared with the team. Tailor entries to this project, but assume only you (this agent, in this clone) will read them back.
