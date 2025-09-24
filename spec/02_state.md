# Vault State

The Vault module persists **vault accounts**, **interest scheduling metadata**, and **swap-out jobs** using typed collections.  
Canonical vault accounts live in `x/auth` (as `VaultAccount`), while this module maintains compact lookups and queues for automated processing.  
Vaults may be configured with an optional **payment denom** in addition to the **underlying asset**; accepted I/O denoms are always the underlying asset and, if set, the payment denom. :contentReference[oaicite:0]{index=0}

---
<!-- TOC -->
- [Canonical Vault Accounts (x/auth)](#canonical-vault-accounts-xauth)
- [Collections (x/vault)](#collections-xvault)
  - [Vault Lookup (prefix 0)](#vault-lookup-prefix-0)
  - [Payout Verification Set (prefix 1)](#payout-verification-set-prefix-1)
  - [Payout Timeout Queue (prefix 2)](#payout-timeout-queue-prefix-2)
  - [Pending Swap-Out Queue (prefix 3)](#pending-swapout-queue-prefix-3)
  - [Pending Swap-Out Sequence (prefix 4)](#pending-swapout-sequence-prefix-4)
  - [Pending Swap-Out by Vault Index (prefix 5)](#pending-swapout-by-vault-index-prefix-5)
  - [Pending Swap-Out by ID Index (prefix 6)](#pending-swapout-by-id-index-prefix-6)
- [Deterministic Vault Addressing](#deterministic-vault-addressing)
- [Genesis Notes](#genesis-notes)

---

## Canonical Vault Accounts (x/auth)

Each vault is an `x/auth` account implementing `VaultAccountI`. The canonical record contains:

- Admin address, share denom, underlying asset, optional **payment denom** (must differ from underlying)  
- Interest configuration: `CurrentInterestRate`, `DesiredInterestRate`, optional `MinInterestRate`/`MaxInterestRate` bounds  
- Swap toggles, `WithdrawalDelaySeconds`, pause flags/reason and `PausedBalance` snapshot

`VaultAccount` enforces invariants (e.g., payment denom cannot equal underlying, rate bounds, etc.) and provides helpers like `AcceptedDenoms()` and `ValidateAcceptedDenom`. :contentReference[oaicite:1]{index=1}

> Note: Because vaults are first-class accounts, the **authoritative storage** for the account itself is `x/auth`. The `x/vault` module adds lookups and queues to operate on those accounts efficiently.

---

## Collections (x/vault)

The module uses typed collections with fixed **prefix IDs** for clarity and upgrade stability. :contentReference[oaicite:2]{index=2}

### Vault Lookup (prefix 0)

A compact lookup keyed by vault address. Used to enumerate vaults and cache serialized account bytes for fast access.

- **Prefix:** `VaultsKeyPrefix` (0)  
- **Key:** `sdk.AccAddress` (vault address)  
- **Value:** `[]byte` (serialized `VaultAccount`)  
:contentReference[oaicite:3]{index=3}

### Payout Verification Set (prefix 1)

A set of vaults queued for **payout verification** (e.g., after rate changes or reconciliations) before they re-enter timeout rotation.

- **Prefix:** `VaultPayoutVerificationSetPrefix` (1)  
- **Key:** `sdk.AccAddress` (vault address)  
- **Value:** none (keyset)  
:contentReference[oaicite:4]{index=4}

### Payout Timeout Queue (prefix 2)

A time-ordered queue scheduling when a vault should be revisited for **automatic interest reconciliation** or state checks.

- **Prefix:** `VaultPayoutTimeoutQueuePrefix` (2)  
- **Key:** typically `(uint64 timeoutSeconds, sdk.AccAddress vault)` (implementation uses typed queue entries)  
- **Value:** none  
:contentReference[oaicite:5]{index=5}

### Pending Swap-Out Queue (prefix 3)

Holds **withdrawal jobs** created by `SwapOut`. Jobs are processed after the vault’s `WithdrawalDelaySeconds` and include pointers to the vault and the original request.

- **Prefix:** `VaultPendingSwapOutQueuePrefix` (3)  
- **Key:** typically `(int64 dueTime, uint64 id)` to maintain time ordering  
- **Value:** `types.PayoutJob` (wraps the `PendingSwapOut` request and metadata) :contentReference[oaicite:6]{index=6} :contentReference[oaicite:7]{index=7}

### Pending Swap-Out Sequence (prefix 4)

A monotonic sequence used to assign globally unique **request IDs** for pending swap-outs.

- **Prefix:** `VaultPendingSwapOutQueueSeqPrefix` (4)  
- **Value:** last used `uint64` ID (typed by the sequence collection)  
:contentReference[oaicite:8]{index=8}

### Pending Swap-Out by Vault Index (prefix 5)

Reverse index to list all pending requests for a given vault without scanning the entire queue.

- **Prefix:** `VaultPendingSwapOutByVaultIndexPrefix` (5)  
- **Key:** `(sdk.AccAddress vault, uint64 id)`  
- **Value:** none  
:contentReference[oaicite:9]{index=9}

### Pending Swap-Out by ID Index (prefix 6)

Direct lookup by **request ID** (useful to expedite or cancel a single job).

- **Prefix:** `VaultPendingSwapOutByIdIndexPrefix` (6)  
- **Key:** `uint64 id`  
- **Value:** lightweight pointer to the queued entry (implementation detail)  
:contentReference[oaicite:10]{index=10}

---

## Deterministic Vault Addressing

Given a **share denom**, the corresponding vault account address is derived deterministically:  
`addr = AddressHash("vault/<shareDenom>")`. This enables “find the vault by share denom” without maintaining a separate index. :contentReference[oaicite:11]{index=11}

---

## Genesis Notes

The module defines a minimal `GenesisState` with validation and relies on import/export logic to include **vault accounts** (from `x/auth`) and active **queue entries** (timeouts and pending swap-outs). There are **no module Params** in the vault genesis. :contentReference[oaicite:12]{index=12} :contentReference[oaicite:13]{index=13}

---
