# Vault Queries

There are several queries for getting information about vaults, estimating conversions, and inspecting the swap-out queue.  
All endpoints are defined in `vault.v1.Query` and exposed over gRPC and REST (HTTP annotations shown below).

---
<!-- TOC 2 2 -->
  - [Vaults](#vaults)
  - [Vault](#vault)
  - [EstimateSwapIn](#estimateswapin)
  - [EstimateSwapOut](#estimateswapout)
  - [PendingSwapOuts](#pendingswapouts)

---

## Vaults

Returns a paginated list of all vault accounts.

- **gRPC:** `Query/Vaults`
- **REST:** `GET /vault/v1/vaults`

### Request — `QueryVaultsRequest`
- `pagination` *(optional)*: standard Cosmos `PageRequest`.

### Response — `QueryVaultsResponse`
- `vaults`: array of `VaultAccount` (full account records, including admin, denoms, interest config, toggles, pause state, etc.)
- `pagination`: `PageResponse`.

**Notes**
- This enumerates vaults via the keeper’s vault collection; it returns the **authoritative** account objects.
- Use this for discovery and basic listing UIs.

---

## Vault

Returns configuration **and live balances** for a specific vault.

- **gRPC:** `Query/Vault`
- **REST:** `GET /vault/v1/vaults/{id}`

### Request — `QueryVaultRequest`
- `id`: either the vault’s **bech32 address** or its **share denom**.

### Response — `QueryVaultResponse`
- `vault`: `VaultAccount` (the target vault)
- `principal`: `AccountBalance` of the vault’s **principal marker account**  
  - `address`: principal marker address (the marker backing the share denom)  
  - `coins`: all balances on that marker (most relevantly, the underlying asset)
- `reserves`: `AccountBalance` of the **vault account** (used for positive interest payments)

**Notes**
- This endpoint resolves the share marker by **share denom**, then reports principal (marker) and reserve (vault) balances.
- Use this to show current funding, e.g., assets backing shares and interest reserves.

---

## EstimateSwapIn

Estimates how many **shares** would be minted for a given deposit of assets (underlying or the optional payment denom).

- **gRPC:** `Query/EstimateSwapIn`
- **REST:** `GET /vault/v1/vaults/{vault_address}/estimate_swap_in`

### Request — `QueryEstimateSwapInRequest`
- `vault_address`: bech32 vault address.
- `assets`: `Coin` you plan to deposit.  
  - `denom` must be accepted by the vault: `underlying_asset` or the configured `payment_denom` (if set).

### Response — `QueryEstimateSwapInResponse`
- `assets`: `Coin` representing the **estimated shares** to be received (denom = vault’s `share_denom`).
- `height`: block height used for the estimate.
- `time`: UTC block time used for the estimate.

**How it works (high level)**
- Validates the deposit denom is accepted.
- Converts deposit → underlying using current NAV (unit price of deposit denom versus underlying).
- Computes a **pro-rata** share amount based on:
  - total shares outstanding,
  - total principal in the marker,
  - plus **accrued-but-unapplied interest** via `CalculateVaultTotalAssets`.
- Floors where necessary to prevent over-mint.

**Common errors**
- Unsupported deposit denom.
- NAV unavailable for the provided pair.
- Invalid vault address.

---

## EstimateSwapOut

Estimates how many **payout assets** (underlying or payment denom) you would receive for a given amount of **shares**.

- **gRPC:** `Query/EstimateSwapOut`
- **REST:** `GET /vault/v1/vaults/{vault_address}/estimate_swap_out`

### Request — `QueryEstimateSwapOutRequest`
- `vault_address`: bech32 vault address.
- `shares`: amount of shares to redeem (as a string-encoded `Int`).
- `redeem_denom` *(optional)*: payout denom to estimate; if empty, defaults to the vault’s `underlying_asset`.  
  Must be either `underlying_asset` or the configured `payment_denom` (if set).

### Response — `QueryEstimateSwapOutResponse`
- `assets`: `Coin` in the **redeem denom**, representing the estimated payout.
- `height`: block height used.
- `time`: UTC block time used.

**How it works (high level)**
- Validates the requested payout denom is accepted (underlying or payment denom).
- Computes a **pro-rata** redemption of the vault’s estimated total assets (principal + accrued interest), then converts underlying → payout denom using current NAV.
- Floors where necessary for safety.

**Common errors**
- Unsupported payout denom.
- NAV unavailable for the payout pair.
- Invalid vault address.

**Reminder**
- This is a **stateless estimate**; actual `SwapOut` is asynchronous and paid later after the vault’s withdrawal delay. See the Events doc for completion/refund signals.

---

## PendingSwapOuts

Returns a paginated list of all **queued** swap-out requests, with their `request_id` and scheduled timeout.

- **gRPC:** `Query/PendingSwapOuts`
- **REST:** `GET /vault/v1/pending_swap_outs`

### Request — `QueryPendingSwapOutsRequest`
- `pagination` *(optional)*: standard Cosmos `PageRequest`.

### Response — `QueryPendingSwapOutsResponse`
- `pending_swap_outs`: array of `PendingSwapOutWithTimeout`:
  - `request_id`: unique ID assigned at `MsgSwapOut`.
  - `pending_swap_out`: the queued request (includes `vault_address`, `owner`, `shares`, `redeem_denom`, etc.).
  - `timeout`: the scheduled block time at/after which the job is eligible for processing.
- `pagination`: `PageResponse`.

**Usage pattern**
- Users submit `MsgSwapOut`, capture `request_id`, then:
  - Poll `PendingSwapOuts` to watch position/timeouts, **or**
  - Subscribe to events and wait for `EventSwapOutCompleted` or `EventSwapOutRefunded` with the same `request_id`.

**Operational notes**
- An admin may call `MsgExpeditePendingSwapOut` on a `request_id` to accelerate processing. The query result will still show the item until it is completed/refunded.

---
