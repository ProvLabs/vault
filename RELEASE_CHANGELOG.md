## [v1.0.13](https://github.com/provlabs/vault/releases/tag/v1.0.13) 2025-12-09

ProvLabs vault module `v1.0.13` includes a set of small but important improvements across the module.  
This release tightens security checks, resolves a handful of minor bugs reported in recent testing, and cleans up edge-case behavior in interest reconciliation and querying.  

### Bug Fixes

* Fix interest payment calculation for composite principals by using TVV in underlying denom [PR 116](https://github.com/provlabs/vault/pull/116).
* Fix interest payment period on vault unpause [#117](https://github.com/provlabs/vault/issues/117).
* Fix handling GetVault errors in Vaults query [PR 129](https://github.com/provlabs/vault/pull/129).
* Fix circular dependency in autopause [#132](https://github.com/provlabs/vault/issues/132).
* Fix ValidateInterestRateLimits validation logic [#140](https://github.com/provlabs/vault/issues/140).

### Features

* Add changelog entry system [PR 118](https://github.com/provlabs/vault/pull/118).

### Improvements

* Strengthen validation on initial deposit logic in CalculateSharesProRataFraction [#134](https://github.com/provlabs/vault/issues/134).
* Restrict TVV calculation to accepted denoms [#137](https://github.com/provlabs/vault/issues/137).
* Use SetVaultAccount consistently to validate all vault storing [#138](https://github.com/provlabs/vault/issues/138).
* Add CONTRIBUTING.md document to assist 3rd parties [PR 144](https://github.com/provlabs/vault/pull/144).

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.0.12...v1.0.13

