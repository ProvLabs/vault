# Contributing to Vault

Thanks for your interest in contributing to the `provlabs/vault` module.

This document describes the minimal process to get changes into the repo:
you work from **your own fork**, open a **PR against `main`**, and include a
**changelog entry** for each meaningful change.

If anything here is unclear, open an issue or ask in your PR.

---

## 1. Fork-First Workflow

You should not create branches directly on `provlabs/vault`.  
Instead, work from your own fork.

### 1.1 Create and clone your fork

1. Go to: `https://github.com/provlabs/vault`
2. Click **Fork** and create a fork under your GitHub account.
3. Clone your fork:

```bash
   git clone git@github.com:<your-username>/vault.git
   cd vault
```

### 1.2 Add the upstream remote

Point `upstream` at the main `provlabs/vault` repo:

```bash
git remote add upstream git@github.com:provlabs/vault.git
git fetch upstream
```

Now you have:

* `origin` → your fork
* `upstream` → main project (`provlabs/vault`)

### 1.3 Keep your fork in sync

Before starting work:

```bash
git checkout main
git fetch upstream
git reset --hard upstream/main
git push origin main
```

---

## 2. Branching and Local Workflow

All changes should be made on a feature branch in **your fork**, then PR’d
into `upstream/main`.

### 2.1 Branch naming

Use a clear, descriptive branch name.
If there is a GitHub issue, include the issue number.

Examples:

* With an issue:

  * `yourname/123-fix-interest-bug`
  * `yourname/42-add-expiration-tests`
* Without an issue:

  * `yourname/add-vault-docs`
  * `yourname/refactor-interest-math`

### 2.2 Basic flow

1. Ensure `main` is up to date (see 1.3).

2. Create a branch:

   ```bash
   git checkout -b yourname/123-fix-interest-bug
   ```

3. Make your changes and commit them.

4. Add a changelog entry (see [Changelog Entries](#3-changelog-entries)).

5. Push your branch **to your fork**:

   ```bash
   git push origin yourname/123-fix-interest-bug
   ```

6. Open a Pull Request from `your-username/vault:yourname/123-fix-interest-bug`
   → `provlabs/vault:main`.

If the work is not ready yet, open the PR as a **Draft** so others can see
what you’re doing and give early feedback.

---

## 3. Changelog Entries

This repo uses the `.changelog/` directory plus helper scripts to track
unreleased changes.

**Every PR that changes behavior, fixes a bug, or adds a feature must include at least one changelog entry.**

For full detail, see `.changelog/README.md`.
Below is the minimal workflow you need.

### 3.1 Where entries live

Unreleased entries are stored under:

```text
.changelog/unreleased/<section>/<num>-<id>.md
```

You normally won’t create these by hand; use `.changelog/add-change.sh`.

Common section names include:

* `features`
* `improvements`
* `bug-fixes`
* `dependencies`

You can list all valid sections with:

```bash
.changelog/get-valid-sections.sh
# or
make get-valid-sections
```

### 3.2 Using `.changelog/add-change.sh` (recommended)

If your branch is named:

```text
yourname/123-fix-interest-bug
```

You can create a changelog entry with:

```bash
.changelog/add-change.sh bug "Fix interest payout when reserves hit zero"
```

This will:

* Infer the issue number `123` from the branch name.
* Map `bug` → `bug-fixes`.
* Create an entry like:

```text
.changelog/unreleased/bug-fixes/123-fix-interest-bug.md
```

With content similar to:

```md
* Fix interest payout when reserves hit zero [#123](https://github.com/provlabs/vault/issues/123).
```

#### Explicit issue or PR number

If you want to be explicit:

* For an **issue**:

  ```bash
  .changelog/add-change.sh --issue 123 bug "Fix interest payout when reserves hit zero"
  ```

* For a **PR** (e.g. once the PR exists and you know the number):

  ```bash
  .changelog/add-change.sh --pr 456 bug "Fix interest payout when reserves hit zero"
  ```

#### Dependency changes

If your change updates dependencies (`go.mod`, etc.):

```bash
go mod tidy
.changelog/get-dep-changes.sh --pull-request 456 --id bump-deps
```

This will create a `dependencies` entry under `.changelog/unreleased/`.

---

## 4. Pull Request Expectations

Try to keep PRs focused: one logical feature or fix per PR when possible.

Before marking your PR “Ready for Review”:

* [ ] Sync your branch with latest `upstream/main` (rebase if necessary).

* [ ] Ensure tests pass locally:

  ```bash
  go test ./...
  # or the project’s preferred targets, e.g.
  # make test
  # make lint
  ```

* [ ] Add or update tests for new behavior where reasonable.

* [ ] Add at least one changelog entry in `.changelog/unreleased/`.

* [ ] Ensure new public functions/types have accurate GoDoc comments.

Reviewers will check:

* Correctness of logic (especially around vault state, interest, and expiration).
* Test coverage.
* Naming and code clarity.
* No obvious performance or safety issues.

---

## 5. Testing and Style

General preferences:

* Use table-driven tests for multiple input cases.

* Prefer `require` / `assert` (or repo-standard helpers) over raw `t.Fail`.

* Provide useful assertion messages, e.g.:

  ```go
  require.NoError(t, err, "ReconcileVaultInterest should not return an error")
  ```

* Keep code straightforward and idiomatic Go.

---

## 6. Need More Detail?

For the full changelog system design and release-prep flow, read:

```text
.changelog/README.md
```

For most contributions, the process is:

1. Fork the repo and keep `main` synced via `upstream`.
2. Create a feature branch in your fork.
3. Make changes + tests.
4. Add a `.changelog` entry with `add-change.sh`.
5. Push to your fork and open a PR against `provlabs/vault:main`.

```
