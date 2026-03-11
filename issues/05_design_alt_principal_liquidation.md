**Context**

This alternative design addresses the challenge of fee collection when a vault's reserves (interest account) are insufficient or empty. It introduces a "Watermark Pattern" to track accrued fees without additional state and a "Principal Liquidation" mechanism to ensure ProvLabs is paid by drawing from the vault's principal assets if necessary.


**Dependencies**
- **Blocked by:** Confirmation of the final ProvLabs Bech32 group account address.
- **Depends on:** `issues/01_fee_calculation_and_constants.md` for base calculation logic.


**What needs to be done**

- **Implement Watermark Logic:** Refactor the reconciliation process to ensure `PeriodStart` is only updated upon successful completion of all fee and interest transfers.
- **Implement Principal Liquidation:** Develop logic to automatically transfer the fee deficit from the **Principal Marker Account** to the **Vault Reserves** before routing the payment to ProvLabs.
- **Audit NAV Impact:** Implement specific event logging for principal liquidation to provide transparency regarding the resulting drop in NAV-per-share.


**Technical details**

### The Watermark Pattern
The `PeriodStart` field in the `VaultAccount` acts as the "watermark." 
- **Accrual:** Fee = `AUM * 0.0015 * (CurrentBlockTime - PeriodStart) / SecondsPerYear`.
- **Persistence:** If a reconciliation fails (e.g., due to lack of total liquidity), `PeriodStart` remains unchanged. 
- **Outcome:** The next successful reconciliation will automatically calculate the fee for the entire elapsed time, effectively "catching up" on the debt.

### Principal Liquidation Logic (within `keeper/reconcile.go`)

```go
// Proposed Logic Flow:
1. Calculate the total fee owed since PeriodStart.
2. Check Vault Reserve balance (UnderlyingAsset).
3. If Reserves < Fee:
    a. deficit = Fee - Reserves
    b. available_principal = BankKeeper.GetBalance(MarkerAddress, UnderlyingAsset)
    c. transfer_amount = min(deficit, available_principal)
    d. BankKeeper.SendCoins(MarkerAddress -> VaultAddress, transfer_amount)
    e. Emit EventVaultPrincipalLiquidation(vault, transfer_amount)
4. Transfer total Fee from VaultAddress to ProvLabsAddress.
5. If fee transfer succeeds, proceed to interest reconciliation.
6. Only if ALL transfers succeed, update Vault.PeriodStart = CurrentBlockTime.
```

### Event Schema
- **EventVaultPrincipalLiquidation:**
    - `vault_address`: string
    - `amount`: string (e.g., "100nhash")
    - `reason`: "AUM_FEE_LIQUIDATION"
- **EventVaultFeeCollected:**
    - `vault_address`: string
    - `fee_amount`: string
    - `aum_snapshot`: string
    - `period_seconds`: int64


**Acceptance Criteria**
- [ ] Fees are calculated cumulatively from the last successful `PeriodStart`.
- [ ] Deficits in reserves are automatically covered by the principal marker's underlying balance.
- [ ] `EventVaultPrincipalLiquidation` is emitted whenever principal is used to pay fees.
- [ ] NAV-per-share correctly reflects the reduction in principal after a liquidation event.
- [ ] The vault does not enter an inconsistent state if a liquidation or fee transfer fails midway.
