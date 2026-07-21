## [v1.2.1](https://github.com/provlabs/vault/releases/tag/v1.2.1) 2026-07-21

This is a fast-follow patch release to v1.2.0 containing two minor fixes: a genesis fix so a `DefaultAumFeeBips` of zero survives export and import instead of being replaced with the module default, and removal of an unused import from `tx.proto` that tripped downstream `IMPORT_USED` lint checks.

No state migrations are required.

### Improvements

* Removed the unused `provlabs/vault/v1/vault.proto` import from `tx.proto` so downstream consumers linting with `IMPORT_USED` no longer report it [#252](https://github.com/provlabs/vault/issues/252).

### Bug Fixes

* Honor a genesis `DefaultAumFeeBips` of zero in `InitGenesis` instead of silently replacing it with the module default, so params round-trip through genesis export and import [#253](https://github.com/provlabs/vault/issues/253).

### Full Commit History

* https://github.com/provlabs/vault/compare/v1.2.0...v1.2.1

