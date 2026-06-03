## [v1.0.16](https://github.com/provlabs/vault/releases/tag/v1.0.16) 2026-06-03

This release adds overflow protection to interest-rate and NAV calculations, including an absolute interest-rate ceiling and safe multiplication guards that return errors instead of panicking the block hook.

### Improvements

* Add an absolute interest-rate ceiling and overflow-safe interest calculations [PR 205](https://github.com/provlabs/vault/pull/205).

### Bug Fixes

* Guard NAV valuation multiplications with `SafeMul` so an oversized net asset value returns an error instead of panicking the block hook [#206](https://github.com/provlabs/vault/issues/206).

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.0.15...v1.0.16

