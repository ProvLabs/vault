# Contributing to Vault

Thanks for your interest in contributing to the `provlabs/vault` module.

This document describes the minimal process to get changes into the repo:
you work from **your own fork**, open a **PR against `main`**, use
**signed/verified commits**, and include a **changelog entry** plus **tests**
for each meaningful change.

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

Use a clear, descriptive branch name. If there is a GitHub issue, include the issue number.

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

3. Make your changes and commit them (with **signed commits**, see [Signed Commits](#3-signed-commits-required)).

4. Add a changelog entry (see [Changelog Entries](#4-changelog-entries)).

5. Push your branch **to your fork**:

```bash
   git push origin yourname/123-fix-interest-bug
```

6. Open a Pull Request from `your-username/vault:yourname/123-fix-interest-bug`
   → `provlabs/vault:main`.

If the work is not ready yet, open the PR as a **Draft** so others can see
what you’re doing and give early feedback.

---

## 3. Signed Commits (Required)

All commits in PRs to `provlabs/vault` must be **cryptographically signed** and show up as **Verified** on GitHub.

If your commits are not signed, CI / branch protections may block merging, and reviewers will ask you to fix your Git history.

### 3.1 Configure signing locally (high level)

There are multiple ways to sign commits (GPG, SSH, or GitHub-suggested methods). At a high level:

1. Generate or use an existing signing key (GPG or SSH).

2. Add the public key to your GitHub account (under **Settings → SSH and GPG keys**).

3. Tell Git to use that key and sign by default, for example (GPG-style):

```bash
   git config --global user.signingkey <your-key-id>
   git config --global commit.gpgsign true
```

4. Make sure your `user.name` and `user.email` match what GitHub expects for your account.

Once this is configured, new commits will be signed, and GitHub should show them as **Verified** in the commit list and PR history.

### 3.2 Fixing unsigned commits

If you already have unsigned commits on your branch, you’ll need to rewrite them as signed:

```bash
git rebase -i upstream/main
# mark commits as 'edit' as needed, then for each:
git commit --amend -S
git rebase --continue
```

Finally, force-push your updated branch to your fork:

```bash
git push --force-with-lease origin your-branch-name
```

After that, the PR should show only **Verified** commits.

---

## 4. Changelog Entries

This repo uses the `.changelog/` directory plus helper scripts to track
unreleased changes.

**Every PR that changes behavior, fixes a bug, or adds a feature must include at least one changelog entry.**

For full detail, see `.changelog/README.md`.
Below is the minimal workflow you need.

### 4.1 Where entries live

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

### 4.2 Using `.changelog/add-change.sh` (recommended)

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

## 5. Testing

Changes should come with tests, and you should run tests locally before
marking your PR as “Ready for Review”.

There are two main categories of tests:

1. **Unit / integration tests via `go test`**
2. **Simulation (“sims”) tests driven by the CLI**

### 5.1 Unit and integration tests

For most changes under `keeper`, `interest`, `queue`, and `types`:

* Add or update tests in the appropriate package.
* Use table-driven tests where you’re exercising behavior across multiple inputs.
* Prefer `require` / `assert` (or established helpers) with meaningful messages, e.g.:

```go
  require.NoError(t, err, "ReconcileVaultInterest should not return an error")
```

To run the full unit test suite locally:

```bash
go test ./...
```

If you want to focus on a specific package:

```bash
go test ./keeper/...
go test ./interest/...
```

For more thorough coverage you can add flags:

```bash
go test -race -cover ./...
```

If your change significantly alters core logic (e.g., interest calculation,
expiration behavior, or state transitions), please ensure the relevant tests
are in place and that `go test ./...` passes before opening or updating your PR.

### 5.2 Simulation tests (“sims”)

The repo also has simulation tests that exercise the application end-to-end
using different simulation configurations. These are slower but very useful
for catching non-obvious failures or invariants.

The GitHub Actions workflow runs simulations using a matrix of names.
Conceptually, the CI does:

* Build the app:

  ```bash
  make build
  ```

* Run simulation tests for each scenario:

  ```bash
  make test-sim-import-export
  make test-sim-multi-seed-short
  make test-sim-after-import
  make test-sim-simple
  make test-sim-benchmark
  make test-sim-nondeterminism
  make test-sim-benchmark-invariants
  ```

You do **not** have to run every simulation locally for every small change, but:

* If you are modifying core module behavior, app wiring, or invariants,
  it is strongly recommended to run at least one or two of these locally, e.g.:

  ```bash
  make build
  make test-sim-simple
  ```

* For more invasive changes, running additional sims (such as `import-export`
  or `multi-seed-short`) is encouraged before requesting review.

### 5.3 CI and extra verification

When you open or update a PR:

* GitHub will run workflows that:

  * Build the app.
  * Run tests.
  * Run the simulation matrix shown above.

Repo admins/maintainers may also re-run these workflows as an extra verification step before merging.

Your responsibility is to:

* Make sure `go test ./...` passes locally.
* Run any additional tests (including sims) that are appropriate for the scope of your change.
* Fix test failures before marking the PR “Ready for Review”.

---

## 6. Pull Request Expectations

Try to keep PRs focused: one logical feature or fix per PR when possible.

Before marking your PR “Ready for Review”:

* [ ] Sync your branch with latest `upstream/main` (rebase if necessary).

* [ ] Ensure all commits are **signed** and show up as **Verified** on GitHub.

* [ ] Ensure unit tests pass locally:

  ```bash
  go test ./...
  ```

* [ ] For core / risky changes, run at least one sim locally, for example:

  ```bash
  make build
  make test-sim-simple
  ```

* [ ] Add or update tests for new/changed behavior where reasonable.

* [ ] Add at least one changelog entry in `.changelog/unreleased/`.

* [ ] Ensure new public functions/types have accurate GoDoc comments.

Reviewers will check:

* Correctness of logic (especially around vault state, interest, expirations, and invariants).
* Test coverage and meaningful assertions.
* Signed/verified commits and clean history.
* Naming and code clarity.
* No obvious performance or safety issues.

---

## 7. Coding Style Notes

A few general style preferences:

* Use table-driven tests for multiple input cases.
* Prefer `require` / `assert` over raw `t.Fail` / `t.Skip`, unless there’s a good reason.
* Provide useful assertion messages, referencing the function or value under test.
* Keep code straightforward and idiomatic Go; avoid unnecessary cleverness.

---

## 8. Need More Detail?

For the full changelog system design and release-prep flow, read:

```text
.changelog/README.md
```

For most contributions, the process is:

1. Fork the repo and keep `main` synced via `upstream`.
2. Create a feature branch in your fork.
3. Make changes + tests.
4. Ensure commits are signed and show as **Verified** on GitHub.
5. Run `go test ./...` (and, for core changes, a sim like `make test-sim-simple`).
6. Add a `.changelog` entry with `.changelog/add-change.sh`.
7. Push to your fork and open a PR against `provlabs/vault:main`.
8. Let the GitHub workflows run the full verification (including sims) before merge.