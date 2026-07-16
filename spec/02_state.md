# Vault State

The Vault module persists **vault accounts**, **interest scheduling metadata**, and **swap-out jobs** using typed collections.  
Canonical vault accounts live in `x/auth` (as `VaultAccount`), while this module maintains compact lookups and queues for automated processing.  
Vaults are strictly **single-denom**: the **underlying asset** is the only accepted I/O denom. Creation no longer takes a payment denom (the request field has been removed and reserved), and the module's v1→v2 state migration flattened any pre-existing mixed-denom vaults so `payment_denom` always equals `underlying_asset`.

> **Deprecation notice:** The payment denom functionality has been removed. The
> `VaultAccount.payment_denom` state field remains on the wire so the migration can decode
> pre-flatten state and always equals `underlying_asset`; field deletion is deferred to a future
> major release (see `spec/01_concepts.md`).

---
<!-- TOC -->
- [Canonical Vault Accounts (x/auth)](#canonical-vault-accounts-xauth)
- [Collections (x/vault)](#collections-xvault)
  - [Vault Lookup (prefix 0)](#vault-lookup-prefix-0)
  - [Payout Verification Set (prefix 1)](#payout-verification-set-prefix-1)
  - [Payout Timeout Queue (prefix 2)](#payout-timeout-queue-prefix-2)
  - [Vault Fee Timeout Queue (prefix 7)](#vault-fee-timeout-queue-prefix-7)
  - [Pending Swap-Out Queue (prefix 3)](#pending-swap-out-queue-prefix-3)
  - [Pending Swap-Out Sequence (prefix 4)](#pending-swap-out-sequence-prefix-4)
  - [Pending Swap-Out by Vault Index (prefix 5)](#pending-swap-out-by-vault-index-prefix-5)
  - [Pending Swap-Out by ID Index (prefix 6)](#pending-swap-out-by-id-index-prefix-6)
  - [AUM Fee Address (prefix 8)](#aum-fee-address-prefix-8)
  - [Internal NAV Table (prefix 11)](#internal-nav-table-prefix-11)
- [Deterministic Vault Addressing](#deterministic-vault-addressing)
- [Genesis Notes](#genesis-notes)
  - [State Migration (v1 → v2)](#state-migration-v1--v2)

---

## Canonical Vault Accounts (x/auth)

Each vault is an `x/auth` account implementing `VaultAccountI`. The canonical record contains:

- Admin address, share denom, underlying asset, deprecated **payment denom** (inert; always equal to the underlying asset — enforced at creation, by validation, and by the v1→v2 migration for pre-existing vaults)  
- Interest configuration: `CurrentInterestRate`, `DesiredInterestRate`, optional `MinInterestRate`/`MaxInterestRate` bounds  
- Swap toggles, `WithdrawalDelaySeconds`, pause flags/reason and `PausedBalance` snapshot  
- **Swap Limits:** `min_swap_in_value`, `min_swap_out_value`, `max_swap_in_value`, and `max_swap_out_value` (measured in underlying asset)
- **Total supply-of-record:** `total_shares` (authoritative across chains; includes locally and externally held shares)  
- **Bridging controls:** `bridge_address` (the sole authorized external address) and `bridge_enabled` (feature gate)
- **Asset Management:** optional `asset_manager` address with delegated authority; it is also the sole authority for P2P settlement (`AcceptAsset`/`RejectAsset`).
- **AUM Fee State:** `fee_period_start`, `fee_period_timeout`, and `outstanding_aum_fee` (denominated in the underlying asset).
- **NAV Authority:** optional `nav_authority` address authorized to mutate the vault's internal NAV table via `MsgUpdateVaultNAV`; the admin acts as NAV authority when unset.

`VaultAccount` enforces invariants (e.g., valid denoms, `payment_denom` empty or equal to the underlying asset, rate bounds, etc.) and provides helpers like `IsAcceptedDenom` and `ValidateAcceptedDenom`, which accept only the underlying asset.

> Note: Because vaults are first-class accounts, the **authoritative storage** for the account itself is `x/auth`. The `x/vault` module adds lookups and queues to operate on those accounts efficiently.

---

## Collections (x/vault)

The module uses typed collections with fixed **prefix IDs** for clarity and upgrade stability. Bridging introduces no new collections; capacity is computed at runtime from local marker/bank supply vs `total_shares`.

### Vault Lookup (prefix 0)

A compact lookup keyed by vault address. Used to enumerate vaults and cache serialized account bytes for fast access.

- **Prefix:** `VaultsKeyPrefix` (0)  
- **Key:** `sdk.AccAddress` (vault address)  
- **Value:** `[]byte` (serialized `VaultAccount`, including `total_shares`, `bridge_address`, `bridge_enabled`)  


### Payout Verification Set (prefix 1)

A set of vaults queued for **payout verification** (e.g., after rate changes or reconciliations) before they re-enter timeout rotation.

- **Prefix:** `VaultPayoutVerificationSetPrefix` (1)  
- **Key:** `sdk.AccAddress` (vault address)  
- **Value:** none (keyset)  


### Payout Timeout Queue (prefix 2)

A time-ordered queue scheduling when a vault should be revisited for **automatic interest reconciliation** or state checks.

- **Prefix:** `VaultPayoutTimeoutQueuePrefix` (2)  
- **Key:** typically `(uint64 timeoutSeconds, sdk.AccAddress vault)` (implementation uses typed queue entries)  
- **Value:** none  


### Vault Fee Timeout Queue (prefix 7)

A time-ordered queue scheduling when a vault should be revisited for **automatic AUM fee collection**.

- **Prefix:** `VaultFeeTimeoutQueuePrefix` (7)  
- **Key:** `(uint64 timeoutSeconds, sdk.AccAddress vault)`  
- **Value:** none

### Pending Swap-Out Queue (prefix 3)

Holds **withdrawal jobs** created by `SwapOut`. Jobs are processed after the vault’s `WithdrawalDelaySeconds` and include pointers to the vault and the original request.

- **Prefix:** `VaultPendingSwapOutQueuePrefix` (3)  
- **Key:** typically `(int64 dueTime, uint64 id)` to maintain time ordering  
- **Value:** `types.PayoutJob` (wraps the `PendingSwapOut` request and metadata)

### Pending Swap-Out Sequence (prefix 4)

A monotonic sequence used to assign globally unique **request IDs** for pending swap-outs.

- **Prefix:** `VaultPendingSwapOutQueueSeqPrefix` (4)  
- **Value:** last used `uint64` ID (typed by the sequence collection)  


### Pending Swap-Out by Vault Index (prefix 5)

Reverse index to list all pending requests for a given vault without scanning the entire queue.

- **Prefix:** `VaultPendingSwapOutByVaultIndexPrefix` (5)  
- **Key:** `(sdk.AccAddress vault, uint64 id)`  
- **Value:** none  


### Pending Swap-Out by ID Index (prefix 6)

Direct lookup by **request ID** (useful to expedite or cancel a single job).

- **Prefix:** `VaultPendingSwapOutByIdIndexPrefix` (6)  
- **Key:** `uint64 id`  
- **Value:** lightweight pointer to the queued entry (implementation detail)  


### AUM Fee Address (prefix 8)

The address authorized to receive collected AUM technology fees.

- **Prefix:** `AUMFeeAddressKeyPrefix` (8)
- **Key:** none (singleton)
- **Value:** raw `sdk.AccAddress` bytes (prefix-agnostic ProvLabs collection address)

### Internal NAV Table (prefix 11)

Per-vault price entries for asset denoms the vault holds or settles. The vault module is the **sole source of truth** for these values; the valuation engine reads them for TVV/share pricing. Entries are written by the NAV authority (`MsgUpdateVaultNAV`) and by p2p settlements (`MsgAcceptAsset`), which also remove an entry when an outbound settlement drains the denom from the principal.

- **Prefix:** `NAVsKeyPrefix` (11)
- **Key:** `(sdk.AccAddress vault, string denom)`
- **Value:** `types.VaultNAV { denom, price, volume, source, updated_block_height, updated_time }` — `price` is the total value of `volume` units of `denom`; per-unit value is `price / volume`. The `price` denom must be the owning vault's underlying asset.

---

## Deterministic Vault Addressing

Given a **share denom**, the corresponding vault account address is derived deterministically:  
`addr = AddressHash("vault/<shareDenom>")`. This enables “find the vault by share denom” without maintaining a separate index.

---

## Genesis Notes

The module defines a minimal `GenesisState` with validation and relies on import/export logic to include **vault accounts** (from `x/auth`) and active **queue entries** (timeouts and pending swap-outs). There are **no module Params** in the vault genesis.  
Genesis must preserve `total_shares`, `bridge_address`, and `bridge_enabled`, and validate that local marker supply does not exceed `total_shares`.  
Genesis validation also enforces the single-denom model: every NAV entry's `price` denom must equal the owning vault's underlying asset, and `VaultAccount` validation requires `payment_denom` to be empty or equal to the underlying asset.

### State Migration (v1 → v2)

The module's consensus version 1→2 migration flattens any pre-existing mixed-denom vaults into the single-denom model. For each vault it sets `payment_denom = underlying_asset`, re-denominates `outstanding_aum_fee` into the underlying asset, and defaults `nav_authority` to the admin when unset; it also rewrites any pending swap-out's redeem denom to the owning vault's underlying asset. No funds move and no accounts are deleted.

---