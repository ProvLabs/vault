package interest

import (
	"fmt"

	"github.com/provlabs/vault/types"

	sdkmath "cosmossdk.io/math"
)

// CalculateAUMFee calculates the pro-rata AUM fee for a given AUM snapshot and duration.
// Formula: Fee = (AUM * 0.0015 * duration) / 31_536_000
func CalculateAUMFee(aum sdkmath.Int, duration int64) (sdkmath.Int, error) {
	if duration <= 0 {
		return sdkmath.ZeroInt(), nil
	}
	rate, err := sdkmath.LegacyNewDecFromStr(types.AUMFeeRate)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("invalid AUM fee rate: %w", err)
	}

	// Fee = (AUM * rate * duration) / SecondsPerYear
	aumDec := sdkmath.LegacyNewDecFromInt(aum)
	durationDec := sdkmath.LegacyNewDec(duration)
	yearDec := sdkmath.LegacyNewDec(SecondsPerYear)

	feeDec := aumDec.Mul(rate).Mul(durationDec).Quo(yearDec)

	return feeDec.TruncateInt(), nil
}
