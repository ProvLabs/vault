---
name: Mock keepers in utils/mocks
description: Hand-rolled mocks for AuthKeeper, ExchangeKeeper, HoldKeeper used by NewVaultKeeper test factory
type: project
---

`utils/mocks/vault.go` exposes:
- `MockAuthKeeper` (already existed)
- `MockExchangeKeeper` — all four exchange methods return zero responses, no error
- `MockHoldKeeper` — `GetHoldCoin` returns zero coin, `GetAllHolds` returns empty Coins

`NewVaultKeeper(t, ...)` builds a Keeper with all dependencies mocked. Most keeper tests do NOT use this; they use the real wired simapp via `*TestSuite`. Mocks are a fallback for pure-unit testing of leaf logic that doesn't depend on bank/marker state.

**How to apply:** When testing logic in isolation (e.g. `recordDiscoveredNAV`, `getNetAssetValue` fall-through paths) where you want to inject error returns, prefer extending the mock with a function-pointer override (`m.GetAllHoldsFunc = func(...) {...}`) rather than introducing a brand-new mock. The current mocks are zero-stubs; if you need error injection you must extend them.
