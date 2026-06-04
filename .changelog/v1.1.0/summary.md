This release introduces the 15 bps annual AUM technology fee with governance-managed
fee parameters and per-vault rate control, configurable minimum and maximum limits for
swap-in and swap-out operations, and broad numeric hardening across the NAV, AUM, and
interest calculations so that oversized values return errors instead of panicking the
block hook. The `provenance` dependency is repointed from the `provlabs` fork to upstream
`main`, with accompanying Cosmos SDK and Go toolchain bumps.

**Client API breaking:** the `AutoCLI` positional argument order for `CreateVault` and
`SetShareDenomMetadata` was aligned with the proto definitions. Scripts and tooling that
rely on the previous positional order must be updated. See
[#135](https://github.com/provlabs/vault/issues/135) for details.
