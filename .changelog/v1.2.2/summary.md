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
