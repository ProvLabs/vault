# Security

The `x/vault` module custodies user deposits on the Provenance Blockchain: each
vault is an on-chain account backed by a restricted share marker, and the
module holds swap-in principal, escrows shares for pending swap-outs, and pays
interest and fees from marker accounts. Vault code also runs inside consensus
(`BeginBlocker`/`EndBlocker` queues and state migrations), so a defect can halt
the chain, not just a single transaction. Security is a design requirement of
every change in this repository, not a review step at the end. This document
is the working guidance for engineers and coding agents; module behavior is
specified in [`spec/`](spec/).

## Reporting a vulnerability

Report vulnerabilities privately via GitHub Security Advisories on this
repository ("Report a vulnerability"). Do not open a public issue or pull
request for a suspected vulnerability, and do not include exploit details in
commit messages or changelog entries while a fix is coordinated. You should
receive an acknowledgment within a few business days; please allow coordinated
disclosure before publishing. There is no bounty program at this time.

## Project status

The module is released and deployed on Provenance networks (testnet and
mainnet). Third-party security reviews are part of the release process, and
review findings are triaged against current code before they are considered
resolved. State-machine-breaking changes ship behind consensus-version bumps
with in-place migrations.

## Secure development practices

### Consensus and chain-halt safety

- **No panics in ABCI paths.** Code reachable from `BeginBlocker`,
  `EndBlocker`, `InitGenesis`, `ExportGenesis`, or a migration returns errors;
  it never panics on data or arithmetic. A failure processing one vault must
  never halt the chain: prefer pausing the affected vault, emitting a
  structured log, and continuing over erroring the block.
- **Bounded work per block.** Queue processing (payouts, pending swap-outs,
  reconciliation) is capped per block; no code path iterates state without a
  bound. Migrations may only walk small, provably bounded sets.
- **Determinism everywhere.** State iteration goes through `collections`
  ordered walks; no map iteration, wall-clock time, floating point, or
  randomness in state-transition logic. The nondeterminism simulation in CI is
  the backstop, not the guarantee.
- **Never strand escrowed funds.** Failure handling for queued jobs must
  account for shares and coins already escrowed; a critical payout or refund
  failure re-queues or preserves the pending entry rather than dropping it.

### Input validation and safe math

- **Validate at every boundary.** Stateless checks live in `ValidateBasic`
  (addresses, denoms, amount signs, limit ranges); stateful checks live in the
  keeper (marker existence, permissions, vault state). A value that cannot be
  bounded safely is an error, never a best-effort continue.
- **Checked arithmetic only.** Use `cosmossdk.io/math` types with
  overflow-guarded operations; scale into `uint64`-safe ranges before
  conversion; round in the vault's favor so dust cannot be extracted by
  repeated swap-in/swap-out cycles. Interest and NAV math must hold across the
  full input domain, including zero, one base unit, and extreme TVL.
- **Estimates match execution.** Query-side estimation (`EstimateSwapIn`,
  `EstimateSwapOut`) must use the same valuation path as the corresponding
  state transition so users cannot be quoted a value the chain will not honor.

### Authorities and asset gating

- **Every privileged capability is enumerated.** Admin, asset manager, NAV
  authority, and bridge address each have a defined capability set in
  [`spec/`](spec/); adding or widening an authority is a spec-level event, not
  a code detail. Handlers verify the signer against the specific authority the
  action requires, never against "any authority".
- **Marker restrictions are the enforcement boundary.** Share markers are
  restricted; transfers flow through the marker send-restriction checks, and
  vault creation preflights `SendRestrictionFn` so a vault cannot be created
  in a configuration whose fee or payout transfers would later fail. Never
  bypass bank send restrictions with direct store writes.
- **Supply-of-record invariant.** `total_shares` is the authoritative share
  supply across chains; local marker supply must never exceed it. Bridge
  mint/burn paths must preserve this invariant on every transition.
- **Permissionless paths are safe for any caller.** Anyone may trigger
  estimation queries and hold shares; nothing gates a safety property on who
  calls, and no permissionless path may move value to the caller's benefit.

### Wire and state compatibility

- **Proto surface is append-only between major releases.** Released fields are
  deprecated in place (`deprecated = true`, documented as inert) rather than
  deleted; deletion waits for a major release and reserves field numbers and
  names. Never-released fields may be removed outright, with numbers reserved.
- **Migrations ship with the consensus-version bump** that requires them, are
  rehearsed against exported network state before upgrade, and log each
  mutation they perform for auditability.

## Testing and verification

- **Table-driven, exhaustive tests** cover each changed branch, including
  failure paths, with high-context assertion messages
  ([`CLAUDE.md`](CLAUDE.md) has the full conventions).
- **Simulations run in CI** (`make test-sim-simple`,
  `make test-sim-nondeterminism`); simulation operations must generate the
  full allowed input domain, and failures must stay seed-reproducible.
- **Invariants are machine-checked, not narrative.** Share supply-of-record,
  escrow conservation across queue failures, and estimate/execution parity are
  asserted by tests, not prose. A change that touches an invariant updates the
  assertion and the spec together.

## Dependencies and supply chain

- `go.mod`/`go.sum` stay pinned and committed; dependency bumps go through
  review and the changelog tooling rather than drive-by upgrades.
- The `provenance-drift` CI job keeps this module's dependency expectations
  aligned with upstream Provenance, so consensus-affecting version skew is
  caught before release.
- Adding a dependency is a reviewed decision: prefer the standard library,
  Cosmos SDK, or Provenance first-party code, and check maintenance and
  provenance of anything new.
- Never commit credentials or private keys. Local development uses the
  disposable `./local.sh` chain; its keys and mnemonics are throwaway test
  material and must never be reused on a public network.

## Audit readiness

Practices that keep formal review cheap and the trail honest:

- **Spec/code parity.** A behavior change updates [`spec/`](spec/) in the same
  change; the spec is the reviewable statement of intended behavior.
- **Changelog discipline.** Every user-visible or state-machine-affecting
  change carries a `.changelog/unreleased/` entry (CI enforces this), so the
  release notes are the true diff of behavior.
- **Committed generated code.** `make proto-all` output (`api/`, `types/`,
  swagger) is regenerated and committed with interface changes, so the
  reviewed interface is the shipped one.
- **Traceable rationale.** Commits and PRs reference the issue they implement,
  so reviewers can walk requirement to code to test.
