---
name: Vault keeper test suite conventions
description: Reusable helpers and patterns for keeper tests via *TestSuite (suite_test.go)
type: project
---

Keeper tests use a single `*TestSuite` struct embedding suite.Suite with a wired `simapp` and `s.k`.

**Why:** All keeper tests share heavy setup (markers, vaults, accounts, NAVs); helpers prevent re-implementing it per test.

**How to apply:** Before writing setup, search `keeper/suite_test.go` for these helpers:
- `s.requireAddFinalizeAndActivateMarker(coin, admin)` — creates+activates a marker (fungible if amount>1, NFT if amount=1)
- `s.setupBaseVault(underlying, shareDenom)` — creates a vault with no payment denom
- `s.setupSinglePaymentDenomVault(underlying, share, payment, num, den)` — creates vault with payment denom and seeds NAV
- `s.EnsureTechFeeAccount()` — guarantees the tech-fee account exists, returns its address
- `s.CreateAndFundAccount(coin)` — creates account funded with given coin
- `s.bumpHeight()` — advances block height (needed before re-setting NAV at the same height)
- `s.setReverseNAV(underlying, denom, price, vol)` — sets a marker NAV in reverse direction

Test accessors live in `keeper/export_test.go` as `Keeper.TestAccessor_*` methods that wrap unexported funcs.

For NetAssetValues store seeding, use:
```go
s.Require().NoError(s.k.NetAssetValues.Set(s.ctx, collections.Join(vaultAddr, denom), types.VaultNAV{
    Price: sdk.NewInt64Coin(underlying, 1),
    Volume: math.OneInt().String(),
}), "...")
```

Use `FundAccount(s.ctx, s.simApp.BankKeeper, addr, coins)` (in suite_test.go) to bypass marker access checks for funding NFT-marker holders.
