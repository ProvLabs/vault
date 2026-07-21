This is a fast-follow patch release to v1.2.0 containing two minor fixes: a genesis fix so a `DefaultAumFeeBips` of zero survives export and import instead of being replaced with the module default, and removal of an unused import from `tx.proto` that tripped downstream `IMPORT_USED` lint checks.

No state migrations are required.
