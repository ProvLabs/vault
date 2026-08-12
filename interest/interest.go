package interest

import (
	"errors"
	"fmt"

	"github.com/provlabs/vault/utils"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	SecondsPerHour = 3_600
	SecondsPerDay  = 86_400
	SecondsPerYear = 31_536_000

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
func CalculateInterestEarned(principal sdk.Coin, rate string, periodSeconds int64) (amount sdkmath.Int, err error) {
	// Recover any LegacyDec overflow panic so callers receive an error and skip the
	// vault instead of propagating the panic. This guard covers both the ExpDec
	// Maclaurin accumulator and the subsequent p.Mul(eRt) final-amount multiply,
	// neither of which has a SafeMul equivalent in LegacyDec.
	defer func() {
		if rec := recover(); rec != nil {
			amount = sdkmath.Int{}
			err = fmt.Errorf("interest calculation overflow (rate=%s, periodSeconds=%d): %v", rate, periodSeconds, rec)
		}
	}()

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

	// e_rt = e^(rt) using the deterministic Maclaurin series approximation
	eRt, err := utils.ExpDec(rt, EulerPrecision)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("failed to compute e^(rt): %w", err)
	}

	// final amount A = P * e^(rt)
	finalAmount := p.Mul(eRt)

	// interest = A - P
	interestAmountDec := finalAmount.Sub(p)

	// Truncate to an integer amount for the coin, as coins cannot have fractional parts.
	return interestAmountDec.TruncateInt(), nil
}

// CalculateAUMFee computes the technology fee based on the vault's AUM and configured basis points.
//
// Formula:
//
//	Fee = (AUM * (bips / 10000) * duration) / 31536000 (SecondsPerYear)
//
// Returns the fee as a truncated sdkmath.Int.
func CalculateAUMFee(aum sdkmath.Int, bips uint32, duration int64) (fee sdkmath.Int, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			fee = sdkmath.Int{}
			err = fmt.Errorf("aum fee calculation overflow (aum=%s, bips=%d, duration=%d): %v", aum, bips, duration, rec)
		}
	}()

	if aum.IsNegative() {
		return sdkmath.Int{}, errors.New("aum cannot be negative")
	}
	if duration < 0 {
		return sdkmath.Int{}, errors.New("duration cannot be negative")
	}
	if duration == 0 || aum.IsZero() || bips == 0 {
		return sdkmath.ZeroInt(), nil
	}

	rate := sdkmath.LegacyNewDec(int64(bips)).Quo(sdkmath.LegacyNewDec(10_000))

	// Fee = (AUM * rate * duration) / SecondsPerYear
	aumDec := sdkmath.LegacyNewDecFromInt(aum)

	durationDec := sdkmath.LegacyNewDec(duration)
	yearDec := sdkmath.LegacyNewDec(SecondsPerYear)

	feeDec := aumDec.Mul(rate).Mul(durationDec).Quo(yearDec)
	return feeDec.TruncateInt(), nil
}
