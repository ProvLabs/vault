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
- Swap toggles, `WithdrawalDelaySeconds` (capped at `MaxWithdrawalDelay`, two years, by account validation so genesis import and migration cannot exceed the bound the message handlers enforce), pause flags/reason and `PausedBalance` snapshot  
- **Swap Limits:** `min_swap_in_value`, `min_swap_out_value`, `max_swap_in_value`, and `max_swap_out_value` (measured in underlying asset)
- **Total supply-of-record:** `total_shares` (authoritative across chains; includes locally and externally held shares)  
- **Bridging controls:** `bridge_address` (the sole authorized external address) and `bridge_enabled` (feature gate)
- **Asset Management:** optional `asset_manager` address with delegated authority; it is also the sole authority for P2P settlement (`AcceptAsset`/`RejectAsset`).
- **AUM Fee State:** `fee_period_start`, `fee_period_timeout`, and `outstanding_aum_fee` (denominated in the underlying asset).
- **NAV Authority:** optional `nav_authority` address authorized to mutate the vault's internal NAV table via `MsgUpdateVaultNAV` and `MsgRemoveVaultNAV`; the admin acts as NAV authority when unset.

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

A vault mid-interest-cycle is tracked in at most one of this set and the [Payout Timeout Queue](#payout-timeout-queue-prefix-2), never both: `SafeAddPayoutVerification` clears the vault's `period_timeout` and dequeues its timeout entry as it adds the vault here, and `SafeEnqueuePayoutTimeout` is only reached from paths that have already removed the vault from this set. A member therefore always carries `period_start != 0` and `period_timeout = 0`. `GenesisState.Validate` asserts both halves of that invariant. See [Genesis Notes](#genesis-notes) for how membership survives an export/import round trip.


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
- **Key:** `(int64 dueTime, uint64 id, sdk.AccAddress vault)` to maintain time ordering  
- **Value:** `types.PendingSwapOut { owner, vault_address, shares, redeem_denom (deprecated), failure_count }`

`dueTime` is when the request becomes eligible for processing, and it doubles as a retry-after: an attempt that fails and has to leave the request queued increments `failure_count` and re-keys the entry to a later `dueTime` so it cannot hold the front of the queue. See [Retry & Backoff](06_blocker.md#retry--backoff).

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

Per-vault price entries for asset denoms the vault holds or is authorized to acquire. The vault module is the **sole source of truth** for these values; the valuation engine reads them for TVV/share pricing. Entries are written only by the NAV authority (`MsgUpdateVaultNAV`, which requires a paused vault to reprice a denom the vault holds) and removed either by the authority (`MsgRemoveVaultNAV`, restricted to denoms the vault does not hold) or by an outbound `MsgAcceptAsset` that drains the denom from the principal. Settlement never writes a price.

An entry may exist for a denom the vault does not hold: TVV values held balances against this table, so an unheld denom contributes nothing until the asset arrives at the principal marker.

- **Prefix:** `NAVsKeyPrefix` (11)
- **Key:** `(sdk.AccAddress vault, string denom)`
- **Value:** `types.VaultNAV { denom, price, volume, source, updated_block_height, updated_time }` — `price` is the total value of `volume` units of `denom`; per-unit value is `price / volume`. The `price` denom must be the owning vault's underlying asset.

### Materialized Total Vault Value (prefix 12)

Each vault's total value, denominated in its underlying asset. This is **derived state**: authoritative for reads, but always reproducible from the vault's principal balances and its Internal NAV table. It exists so reading TVV costs one store read instead of one per priced denom.

Every path that moves a priced balance or changes a price folds its change into this entry — swap-in, swap-out payout, principal deposit/withdraw, interest and AUM fee transfers, settlement staging, and NAV writes and removals. The walk over the NAV table remains the definition of the number, and a registered invariant asserts the two agree (see [Invariants](#invariants)).

A missing entry is derived and stored on first read rather than treated as an error, so the value is self-healing. Vault creation, genesis import, and the v2→v3 migration all seed it up front so the invariant never observes a gap, and unpausing re-derives it. Because a mutation path reports its delta only after the balance has moved, deriving a missing entry supersedes the delta rather than adding to it.

Creation seeds by deriving rather than assuming zero: a vault's principal address follows from its share denom, so `x/marker` can adopt an account that already holds a balance no later path would report.

- **Prefix:** `TotalValuesKeyPrefix` (12)
- **Key:** `sdk.AccAddress vault`
- **Value:** `math.Int` — the vault's gross total value in `underlying_asset`, before the `outstanding_aum_fee` liability is deducted.

---

## Invariants

Registered with `x/crisis` under the `vault` module route. A broken invariant panics, so these run only on invariant-enabled chains — that is, nodes started with a non-zero `--inv-check-period`. Registration is wired on the crisis keeper directly rather than through the module manager, whose `RegisterInvariants` is a no-op in the current SDK.

Each assertion is one an outside account cannot forge. Anything an unprivileged sender could trigger would turn the invariant into a halt-on-demand, so those conditions are deliberately tolerated.

Each route resolves the vault lookup the same way every other consumer does, so the set of vaults checked cannot drift from the set the migration and genesis import seed. A lookup entry whose address holds no account, or holds something other than a vault account, is inert — nothing else reads it and the vault it names owns no balances, shares, or NAV entries — so it is logged, counted into the invariant's message, and passed over rather than halting the chain over state no operator can act on.

| Route | Assertion |
| --- | --- |
| `total-value` | Each vault's materialized total value equals the value derived by walking its NAV table and principal balances. Drift means some mutation path failed to report its change, so share pricing is running off a stale number. A vault whose walk cannot produce a number at all is logged and passed over, since there is no reference to compare against. |
| `share-supply` | No vault's local share supply exceeds its `total_shares`. `total_shares` is the cross-chain supply-of-record that bridge mints are gated on, so local supply overtaking it means the bridge can mint shares nothing backs. |
| `escrowed-shares` | A vault holds at least the shares its pending swap-outs account for. A shortfall means a payout can no longer be honored from escrow. A surplus is tolerated: share transfers to the vault account are unrestricted, so anyone can create one with a bank send. |

---

## Deterministic Vault Addressing

Given a **share denom**, the corresponding vault account address is derived deterministically:  
`addr = AddressHash("vault/<shareDenom>")`. This enables “find the vault by share denom” without maintaining a separate index.

---

## Genesis Notes

The module defines a minimal `GenesisState` with validation and relies on import/export logic to include **vault accounts** (from `x/auth`) and active **queue entries** (timeouts and pending swap-outs). It also carries the module **Params** (`tech_fee_address`, `default_aum_fee_bips`, `gov_only_vault_creation`); an omitted `tech_fee_address` falls back to the chain-specific default, and the other two take their genesis values as given, so a chain that wants governance-gated vault creation must set `gov_only_vault_creation` in genesis or with an `UpdateParams` proposal.  
Genesis must preserve `total_shares`, `bridge_address`, and `bridge_enabled`. `InitGenesis` enforces the `total_shares >= local marker supply` invariant per vault — the check lives there, not in `GenesisState.Validate`, because only the keeper can read x/bank's supply — and panics on an import that violates it. Migrations carry the same obligation: they must never lower `total_shares` below the local supply of the share denom.  
Genesis validation also enforces the single-denom model: every NAV entry's `price` denom must equal the owning vault's underlying asset, and `VaultAccount` validation requires `payment_denom` to be empty or equal to the underlying asset.

`GenesisState` carries `payout_verification_set` so [Payout Verification Set](#payout-verification-set-prefix-1) membership round-trips, and `InitGenesis` additionally *re-derives* it: any imported vault that is unpaused, has `period_start != 0` and `period_timeout = 0`, and holds no payout timeout entry is restored to the set. Deriving it repairs genesis files exported before the field existed, where a vault in the set landed in neither structure on import — no blocker visited it again, so its interest kept accruing while the affordability check that zeroes an unaffordable rate never ran. The derivation is idempotent, so a genesis that carries the field produces the same membership.

Validation deliberately does not require that *every* vault with an open accrual period appear in one of the two structures. `handleDepletedVaults` zeroes a vault's interest rate without clearing its period, so a depleted vault legitimately exports with `period_start != 0` and no entry in either; `InitGenesis` re-arms it rather than rejecting the import.

### State Migration (v1 → v2)

The module's consensus version 1→2 migration flattens any pre-existing mixed-denom vaults into the single-denom model. For each vault it sets `payment_denom = underlying_asset`, re-denominates `outstanding_aum_fee` into the underlying asset, and defaults `nav_authority` to the admin when unset; it also rewrites any pending swap-out's redeem denom to the owning vault's underlying asset. No funds move and no accounts are deleted.

### State Migration (v2 → v3)

The module's consensus version 2→3 migration materializes every vault's total value (prefix 12), which became module state alongside the materialized-TVV read path. It derives each total from current balances and the NAV table and stores it. No funds move and no vault configuration changes.

Seeding is required rather than optional: `GetTVV` would repair each vault lazily on first read, but the `total-value` invariant reads state without repairing it, so an unseeded vault would report as broken — and because `x/crisis` panics on a broken invariant, an invariant-enabled chain would halt. The migration is idempotent, since it recomputes from the same source on every run.

It seeds from the vault lookup, resolving it through the same path the invariant uses, so the two cannot disagree about which vaults must have an entry. A vault it cannot value is logged and skipped: such a vault fails the invariant's own derivation too, so seeding could not have satisfied it, and aborting the upgrade over one bad vault would be worse than leaving the rest of the chain seeded. A lookup entry that resolves to no vault account is skipped by both for the same reason (see [Invariants](#invariants)).

---