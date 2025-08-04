package interest

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/utils"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	SecondsPerHour  = 3_600
	SecondsPerDay   = 86_400
	SecondsPerMonth = 2_628_000
	SecondsPerYear  = 31_536_000

	// TODO What should this value be, and does it need to be hardcoded?
	CalculatePeriodsLimit   = 2 * SecondsPerMonth
	CalculatePeriodsNoLimit = 0

	EulerPrecision = 17
)

// CalculateInterestEarned computes the continuously compounded interest for a given principal over a period.
//
// It uses the formula `Interest = P * (e^(rt)) - P`, where:
//   - P is the `principal`.
//   - r is the annual `rate`.
//   - t is the time in years, derived from (`periodSeconds` / 31_536_000).
//
// This function returns the interest as a `cosmosmath.Int`, truncating any fractional part to ensure compatibility with coin amounts.
// It uses deterministic arithmetic via `cosmosmath.LegacyDec` and approximates e^x using a Maclaurin series (`utils.ExpDec`).
func CalculateInterestEarned(principal sdk.Coin, rate string, periodSeconds int64) (sdkmath.Int, error) {
	r, err := sdkmath.LegacyNewDecFromStr(rate)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("invalid rate string: %w", err)
	}

	if periodSeconds <= 0 {
		return sdkmath.Int{}, errors.New("periodSeconds must be positive")
	}

	// P = principal amount as a deterministic decimal
	p := sdkmath.LegacyNewDecFromInt(principal.Amount)

	// t = time in years, as a deterministic decimal
	t := sdkmath.LegacyNewDec(periodSeconds).QuoInt64(SecondsPerYear)

	// rt
	rt := r.Mul(t)

	// e_rt = e^(rt) using the deterministic Exp function from the SDK
	eRt := utils.ExpDec(rt, EulerPrecision)

	// final amount A = P * e^(rt)
	finalAmount := p.Mul(eRt)

	// interest = A - P
	interestAmountDec := finalAmount.Sub(p)

	// Truncate to an integer amount for the coin, as coins cannot have fractional parts.
	return interestAmountDec.TruncateInt(), nil
}

// CalculateExpiration determines the epoch time at which a vault will no longer be
// able to pay the required interest on a principal amount.
//
// It simulates the compounding process period by period, tracking the vault's
// reserve balance. The expiration time is the start time of the first period for which
// the earned interest exceeds the vault's remaining reserves.
//
// Parameters:
//   - principal: The initial amount of the asset earning interest.
//   - vaultReserves: The funds available in the vault to pay out as interest.
//   - rate: The annual interest rate (e.g., "0.04" for 4%).
//   - periodSeconds: The compounding interval in seconds.
//   - startTime: The epoch second when the interest calculation begins.
//
// Returns:
// - The expiration time as an epoch second.
// - If the vault will never be depleted (e.g., zero interest rate), it returns the startTime.
// - An error if the inputs are invalid or a calculation fails.
func CalculateExpiration(principal sdk.Coin, vaultReserves sdk.Coin, rate string, periodSeconds, startTime, limit int64) (int64, error) {
	if principal.Denom != vaultReserves.Denom {
		return 0, fmt.Errorf("principal and vault denoms must match, got %s and %s", principal.Denom, vaultReserves.Denom)
	}
	if periodSeconds <= 0 {
		return 0, errors.New("periodSeconds must be positive")
	}
	if startTime <= 0 {
		return 0, errors.New("startTime must be positive")
	}
	rateDec, err := sdkmath.LegacyNewDecFromStr(rate)
	if err != nil {
		return 0, fmt.Errorf("invalid rate string: %w", err)
	}
	if rateDec.IsNegative() {
		return 0, errors.New("rate cannot be negative")
	}

	// If the rate is zero, then reserves are not depleted and it never expires.
	if rateDec.IsZero() || principal.IsZero() {
		return startTime, nil
	}

	// Iteratively calculate interest until the vault is depleted.
	periods, i, err := CalculatePeriods(vaultReserves, principal, rate, periodSeconds, limit)
	if err != nil {
		return i, err
	}

	// Calculate final expiration time with overflow checks using int64. Check for multiplication overflow.
	totalSeconds := sdkmath.NewInt(periodSeconds)
	totalSeconds, err = totalSeconds.SafeMul(sdkmath.NewInt(periods))
	if err != nil {
		return 0, fmt.Errorf("failed to calculate total seconds: %w", err)
	}

	expirationTime := sdkmath.NewInt(startTime)
	expirationTime, err = expirationTime.SafeAdd(totalSeconds)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate expiration time: %w", err)
	}

	return expirationTime.Int64(), nil // Unix epoch time
}

// CalculatePeriods simulates continuous compounding to determine how many compounding
// periods a vault can sustain paying interest before its reserves are depleted.
//
// The interest for each period is calculated using the continuous compounding formula,
// and added or subtracted from the principal depending on the sign of the rate.
//
// Parameters:
//   - vaultReserves: The available funds in the vault to pay interest.
//   - principal: The initial amount earning interest.
//   - rate: The annual interest rate (as a string, e.g. "0.05" for 5%, "-0.05" for -5%).
//   - periodSeconds: The length of each compounding period in seconds.
//   - limit: The maximum total duration (in seconds) to simulate. Once the total simulated
//     time reaches or exceeds this limit, the loop stops, even if reserves are not fully depleted.
//     A limit of 0 means no limit.
//
// Returns:
//   - The number of full compounding periods the reserves can cover (up to the limit).
//   - A placeholder value (currently always zero) for potential future use.
//   - An error if interest calculation fails or input is invalid.
func CalculatePeriods(
	vaultReserves sdk.Coin,
	principal sdk.Coin,
	rate string,
	periodSeconds int64,
	limit int64,
) (int64, int64, error) {
	if vaultReserves.Amount.IsNegative() || principal.Amount.IsNegative() {
		return 0, 0, fmt.Errorf("vault reserves and principal must be non-negative")
	}
	if principal.Amount.IsZero() || periodSeconds <= 0 {
		return 0, 0, nil
	}
	if vaultReserves.Denom != principal.Denom {
		return 0, 0, fmt.Errorf("denom mismatch: reserves=%s principal=%s", vaultReserves.Denom, principal.Denom)
	}

	var periods int64
	var totalDuration int64

	for limit == CalculatePeriodsNoLimit || totalDuration <= limit-periodSeconds {
		interest, err := CalculateInterestEarned(principal, rate, periodSeconds)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to calculate interest for period %d: %w", periods+1, err)
		}

		if interest.IsZero() {
			break
		}

		if interest.IsPositive() {
			if vaultReserves.Amount.LT(interest) {
				break
			}
			vaultReserves = vaultReserves.Sub(sdk.NewCoin(vaultReserves.Denom, interest))
			principal = principal.Add(sdk.NewCoin(principal.Denom, interest))
		} else {
			principal = principal.Sub(sdk.NewCoin(principal.Denom, interest.Abs()))
			if principal.Amount.IsNegative() {
				break
			}
		}

		periods++
		totalDuration += periodSeconds
	}

	return periods, 0, nil
}
