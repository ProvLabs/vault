## [v1.0.14](https://github.com/provlabs/vault/releases/tag/v1.0.14) 2026-01-16

This release is mostly a collection of small fixes and cleanups to make the vault module safer and more consistent. The main feature is a new transaction for updating the vault withdrawal delay, but most changes focus on tightening validation, improving address parsing, and fixing a few edge-case bugs around pricing, swaps, and NAV handling. There are no major behavioral changes, just incremental improvements to correctness and reliability.

### Features

* Add tx to update vault withdrawal delay [#164](https://github.com/provlabs/vault/issues/164).

### Improvements

* Replace MustAccAddressFromBech32 with safe address parsing [#121](https://github.com/provlabs/vault/issues/121).
* Standardize Zero Checks in UnitPriceFraction [#133](https://github.com/provlabs/vault/issues/133).
* Add additional vault account setting validation [PR 151](https://github.com/provlabs/vault/pull/151).
* Payment denom is set to underlying on single asset vaults, Swap out's redeem denom defaults to payment denom [PR 157](https://github.com/provlabs/vault/pull/157).

### Bug Fixes

* Fix `QueryEstimateSwapOutRequest` shares custom type error [PR 158](https://github.com/provlabs/vault/pull/158).
* Add uint64 check on publishing of nav volume [#162](https://github.com/provlabs/vault/issues/162).

### Dependencies

* `github.com/cosmos/cosmos-sdk` bumped to v0.50.14 (from v0.50.0) [PR 153](https://github.com/provlabs/vault/pull/153). 

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.0.13...v1.0.14

