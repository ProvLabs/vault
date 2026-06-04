## [v1.1.0](https://github.com/provlabs/vault/releases/tag/v1.1.0) 2026-06-04

This release introduces the 15 bps annual AUM technology fee with governance-managed
fee parameters and per-vault rate control, configurable minimum and maximum limits for
swap-in and swap-out operations, and broad numeric hardening across the NAV, AUM, and
interest calculations so that oversized values return errors instead of panicking the
block hook. The `provenance` dependency is repointed from the `provlabs` fork to upstream
`main`, with accompanying Cosmos SDK and Go toolchain bumps.

**Client API breaking:** the `AutoCLI` positional argument order for `CreateVault` and
`SetShareDenomMetadata` was aligned with the proto definitions. Scripts and tooling that
rely on the previous positional order must be updated. See
[#135](https://github.com/provlabs/vault/issues/135) for details.

### Features

* Implemented configurable minimum and maximum limits for swap-in and swap-out operations [#39](https://github.com/provlabs/vault/issues/39).
* Add gemini file agent persona file with update test [PR 176](https://github.com/provlabs/vault/pull/176).
* Implement 15 bps annual AUM technology fee collected from vault principal [PR 178](https://github.com/provlabs/vault/pull/178).
* Add Claude Code agent personas and skills for code review, testing, and Cosmos SDK / Provenance workflows [PR 196](https://github.com/provlabs/vault/pull/196).

### Improvements

* Ensure atomicity in withdrawal processing via CacheContext to prevent inconsistent states during failures [#131](https://github.com/provlabs/vault/issues/131).
* Aligned `AutoCLI` positional argument order with proto definitions for `CreateVault` (`[#135](https://github.com/provlabs/vault/issues/135).
* Replaced all `ctx.Logger()` calls in the keeper package with the module-scoped `k.getLogger(ctx)` for consistent, structured logging attributed to `x/vault`, and documented the convention in `GEMINI.md` [#179](https://github.com/provlabs/vault/issues/179).
* Implemented global governance parameters for AUM fees and granular per-vault fee rate (bips) management, migrating hardcoded fee logic to a formal Params structure with authorized update capabilities [#180](https://github.com/provlabs/vault/issues/180).
* Convert the `.claude/skills/*/scripts/` helpers (pr-review, branch-diff-analysis, vault-lint-test) from Bash + `jq` to Python 3 stdlib equivalents [PR 200](https://github.com/provlabs/vault/pull/200).
* Add an absolute interest-rate ceiling and overflow-safe interest calculations [PR 205](https://github.com/provlabs/vault/pull/205).
* Document and add test coverage for the bridge mint/burn supply-of-record model, where bridge operations never mutate the vault's `TotalShares` [#207](https://github.com/provlabs/vault/issues/207).
* Add safe-math operations to AUM calculations so overflows return an error instead of panicking [PR 210](https://github.com/provlabs/vault/pull/210).

### Bug Fixes

* Guard NAV valuation multiplications with `SafeMul` so an oversized net asset value returns an error instead of panicking the block hook [#206](https://github.com/provlabs/vault/issues/206).

### Dependencies

* Go bumped to 1.25.8 (from 1.24.1) to match the Go version required by upstream Provenance [#188](https://github.com/provlabs/vault/issues/188).
* `github.com/cosmos/cosmos-sdk` bumped to v0.53.6 (from v0.53.5) [#188](https://github.com/provlabs/vault/issues/188).
* `github.com/provenance-io/provenance` repointed from the `github.com/provlabs/provenance` fork to upstream `main` at `v1.3.2-0.20260519172448-9b0fb12c99b3` (commit `9b0fb12c`) [#188](https://github.com/provlabs/vault/issues/188).

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.0.15...v1.1.0

