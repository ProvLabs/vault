## [v1.2.3](https://github.com/provlabs/vault/releases/tag/v1.2.3) 2026-07-29

This release makes vault creation a governance action.
`MsgCreateVaultRequest` is now signed by the `authority` field, which must be
the governance module account, so a vault can only come into existence through
a passed proposal. The `admin` field is retained and names the administrator
the proposal chose, so the administrator no longer has to be the signer, and
any other signer is rejected as unauthorized. The `tx vault create` CLI command
gained a leading `[authority]` argument to match, and is meant to be run with
`--generate-only` so the resulting message can be submitted as a proposal.

It also contains the internal NAV table to the vault that owns it. The vault no
longer mirrors internal NAV entries into the marker module, which had let a
vault overwrite the chain-wide NAV of any marker it holds without holding a
permission on that marker. The vault's own share denom is now the only marker
NAV it publishes, and a NAV volume above the uint64 range is accepted rather
than rejected. A value-owner NAV denom (`nft/<scope-id>`) must now name a scope
that exists, so a fabricated scope id cannot be priced, and a genesis import
skips an entry whose denom is no longer registered instead of panicking on
restart.

### Bug Fixes

* Require a value-owner NAV denom (`nft/<scope-id>`) to name a scope that exists, so a fabricated scope id cannot be priced. A genesis import skips an entry whose denom is no longer registered instead of panicking [PR 266](https://github.com/provlabs/vault/pull/266).
* Stop mirroring internal NAV entries into the marker module, which let a vault overwrite the chain-wide NAV of any marker it holds no permission on. The vault's share denom is now the only marker NAV published, and a NAV volume above the uint64 range is accepted [PR 266](https://github.com/provlabs/vault/pull/266).

### Client Breaking

* Changed the `tx vault create` CLI command to take `[PR 265](https://github.com/provlabs/vault/pull/265).

### State Machine Breaking

* Restricted vault creation to governance: `MsgCreateVaultRequest` is now signed by the `authority` field, which must be the governance module account, so a vault can only come into existence through a passed governance proposal. The `admin` field is retained and designates the vault administrator chosen by the proposal, so it no longer has to be the signer. Any other signer is rejected with an unauthorized error [PR 265](https://github.com/provlabs/vault/pull/265).

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.2.2...v1.2.3

