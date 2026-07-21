This release introduces the internal NAV system and the peer-to-peer multi-asset
vault workflow, allowing a vault to accept external assets in exchange for its
underlying asset through `x/exchange` settlement, priced against a per-vault NAV
table managed by a designated NAV authority.

It also removes the mixed-denom (payment denom) vault functionality in favor of
the p2p workflow: vaults are now strictly single-denom on their underlying asset,
with a v1 to v2 migration that flattens existing mixed-denom vaults in place.
Vault share markers now require deposit access, restricting principal deposits to
the vault itself.
