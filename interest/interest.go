package interest

import (
	"github.com/provlabs/vault/utils"

	cosmosmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Period struct {
}

const (
	SecondsPerYear = 31_536_000
	EulerPrecision = 18
)

// CalculateInterestEarned computes the continuously compounded interest for a given principal over a period.
//
// It uses the formula `Interest = (P * (e^(rt)) - P`, where:
//   - P is the `principal`.
//   - r is the annual `rate`.
//   - t is the time in years, derived from (`periodSeconds` / 31_536_000).
//
// To ensure on-chain determinism, it uses `cosmosmath.LegacyDec` for all arithmetic
// and approximates e^x using a Maclaurin series (`utils.ExpDec`).
func CalculateInterestEarned(principal sdk.Coin, rate string, periodSeconds int64) (sdk.Coin, error) {
	r, err := cosmosmath.LegacyNewDecFromStr(rate)
	if err != nil {
		return sdk.Coin{}, err
	}

	// P = principal amount as a deterministic decimal
	p := cosmosmath.LegacyNewDecFromInt(principal.Amount)

	// t = time in years, as a deterministic decimal
	t := cosmosmath.LegacyNewDec(periodSeconds).QuoInt64(SecondsPerYear)

	// rt
	rt := r.Mul(t)

	// e_rt = e^(rt) using the deterministic Exp function from the SDK
	eRt := utils.ExpDec(rt, EulerPrecision)

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
