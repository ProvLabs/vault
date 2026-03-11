**Context**

A core component of the AUM-based fee system is the accurate calculation of the 15 bps technology fee based on the vault's Total Vault Value (TVV/AUM). This task involves defining the hard-coded constants for the fee rate and recipient address, and implementing the mathematical logic for linear fee accrual.


**Dependencies**
- **Blocked by:** Confirmation of the final ProvLabs Bech32 group account address.
- **Depends on:** `interest/interest.go` (standard math libraries).


**What needs to be done**

- **Define Constants:** Add `AUMFeeRate = "0.0015"` (15 bps) and `ProvLabsFeeAddress` as immutable constants in the `types/` package.
- **Implement Fee Function:** Create `CalculateAUMFee(aum sdkmath.Int, duration int64)` in `interest/interest.go`.
- **Unit Testing:** Add comprehensive tests in `interest/interest_test.go` for zero AUM, maximum TVV, and fractional day durations.


**Technical details**

- **Formula:** `Fee = (AUM * 0.0015 * duration) / 31536000` (SecondsPerYear).
- **Precision:** Use `sdkmath.LegacyDec` for intermediate calculations to avoid premature truncation.
- **Recipient Address:** Use the Bech32 prefix "provlabs" for the hard-coded wallet address.


**Acceptance Criteria**
- [ ] `AUMFeeRate` is defined as a 15 bps decimal string.
- [ ] `CalculateAUMFee` returns a correctly rounded (truncated) `sdkmath.Int` for any duration.
- [ ] Unit tests pass for various AUM levels and durations.
