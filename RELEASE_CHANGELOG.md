## [v1.2.2](https://github.com/provlabs/vault/releases/tag/v1.2.2) 2026-07-27

This release tightens the authorization model around the internal NAV table.
The NAV authority must now price an asset denom before the asset manager can
acquire it, so the table doubles as the list of denoms a vault is allowed to
take in and at what price, and settlement no longer writes prices of its own.
A new `RemoveVaultNAV` transaction lets the authority revoke that authorization
for denoms the vault does not hold, and both `UpdateVaultNAV` and
`RemoveVaultNAV` are now permitted while a vault is paused so an operator can
pause, reprice, and unpause as a single sequence with user swaps closed for the
whole span.

It also restores fairness in the block-time queues: paused vaults are dropped
from the payout, fee, and verification queues instead of consuming the per-block
visit budget, and a swap-out that fails is re-keyed to a later retry time rather
than holding the front of the queue and re-consuming the batch budget every
block. Rounding out the release, a paused vault no longer accrues AUM fees for
the paused span, and a genesis export carrying a metadata value-owner NAV denom
is importable again instead of panicking on restart.

### Features

* Added a `RemoveVaultNAV` transaction that lets the NAV authority delete a vault internal NAV entry, revoking the authorization to acquire that denom at that price. Only denoms the vault does not hold may be removed, since dropping a held asset entry would erase that balance from total vault value rather than restate it; a held asset that has lost its value is written down through `UpdateVaultNAV` instead. `EventNAVRemoved` gained a `signer` field recording the NAV authority that removed the entry, left empty when the protocol removes it as an outbound settlement draining the denom does [PR 259](https://github.com/provlabs/vault/pull/259).

### Improvements

* Remove paused vaults from the payout timeout, fee timeout, and payout verification queues so they no longer consume the per-block visit budget and starve active vaults [PR 257](https://github.com/provlabs/vault/pull/257).

### Bug Fixes

* Reset the AUM fee period on pause and unpause so a paused vault accrues no fees for the paused span [#219](https://github.com/provlabs/vault/issues/219).
* Re-key a pending swap-out to a later retry time whenever an attempt fails and the request has to stay queued, so a request whose refund fails deterministically no longer holds the front of the queue and re-consumes the per-block batch budget on every block [PR 258](https://github.com/provlabs/vault/pull/258).
* Allow `InitGenesis` to import an internal NAV entry for a metadata value-owner denom (`nft/<scope-id>`), which cannot be registered as a marker. `InitGenesis` and `SetVaultNAV` now share one denom check, so a genesis export carrying a scope asset price is importable on a chain restart or a state-export upgrade instead of panicking [PR 260](https://github.com/provlabs/vault/pull/260).

### State Machine Breaking

* Require the vault NAV authority to price an asset denom before the asset manager can acquire it: `AcceptAsset` now rejects a settlement whose asset denom has no internal NAV entry instead of letting the first acquisition set the price. Settlement no longer writes the NAV table at all, since the exact-price guardrail has already proven the trade executed at the price the authority recorded, so the price stays attributed to the authority and no marker NAV is republished. The internal NAV table now doubles as the list of denoms a vault is authorized to acquire and at what price: the authority may price a denom the vault does not hold yet, and until the asset arrives at the principal marker that entry contributes nothing to total vault value. A first acquisition therefore takes `UpdateVaultNAV` followed by `AcceptAsset`, which can ride in a single transaction carrying both signatures when the two roles belong to different entities [PR 259](https://github.com/provlabs/vault/pull/259).
* Allowed `UpdateVaultNAV` and `RemoveVaultNAV` while a vault is paused, so an operator can pause, reprice held assets, and unpause as one sequence with user swaps closed for the whole span. A repricing during a pause leaves `PausedBalance` frozen, matching the principal deposits and withdrawals that are themselves only allowed while paused; everything done during the pause takes effect together at unpause [PR 262](https://github.com/provlabs/vault/pull/262).

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.2.1...v1.2.2

