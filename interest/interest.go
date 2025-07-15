package interest

import (
	cosmosmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Period struct {
}

const (
	SecondsPerYear = 31_536_000
	EulerPrecision = 18
)

// Calculate the annual interest rate
func CalculateInterest(principal sdk.Coin, rate string, seconds int64) (sdk.Coin, error) {
	r, err := cosmosmath.LegacyNewDecFromStr(rate)
	if err != nil {
		return sdk.Coin{}, err
	}

	// P = principal amount as a deterministic decimal
	p := cosmosmath.LegacyNewDecFromInt(principal.Amount)

	// t = time in years, as a deterministic decimal
	t := cosmosmath.LegacyNewDec(seconds).QuoInt64(SecondsPerYear)

	// rt
	rt := r.Mul(t)

	// e_rt = e^(rt) using the deterministic Exp function from the SDK
	eRt := ExpDec(rt, EulerPrecision)

	// final amount A = P * e^(rt)
	finalAmount := p.Mul(eRt)

	// interest = A - P
	interestAmountDec := finalAmount.Sub(p)

	// Truncate to an integer amount for the coin, as coins cannot have fractional parts.
	interestAmountInt := interestAmountDec.TruncateInt()

	return sdk.NewCoin(principal.Denom, interestAmountInt), nil
}

func CalculateExpiration() uint64 {
	return 0
}

// ExpDec calculates e^x using Maclaurin series expansion up to `terms` terms.
// x must be an cosmosmath.LegacyDec. The more terms, the more accurate (and slower).
// Safe for on-chain use (fully deterministic).
func ExpDec(x cosmosmath.LegacyDec, terms int) cosmosmath.LegacyDec {
	result := cosmosmath.LegacyOneDec()    // starts at 1
	power := cosmosmath.LegacyOneDec()     // x^0
	factorial := cosmosmath.LegacyOneDec() // 0! = 1

	for i := 1; i <= terms; i++ {
		power = power.Mul(x)                     // x^i
		factorial = factorial.MulInt64(int64(i)) // i!
		term := power.Quo(factorial)             // x^i / i!
		result = result.Add(term)
	}

	return result
}
