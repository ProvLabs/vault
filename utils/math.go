package utils

import (
	"fmt"

	"cosmossdk.io/math"
)

// ExpDec calculates e^x using a Maclaurin series expansion of `terms` terms.
// Fully deterministic and safe for on-chain use.
//
//	e^x = 1 + x + x^2/2! + x^3/3! + ... + x^n/n!
//
// The truncated series is only accurate for small |x|, so x is first range-reduced by
// repeated halving until |x| <= 1 and the series result is squared back up
// (e^x = (e^(x/2^k))^(2^k)). Without that reduction the series diverges for large
// positive x and sign-inverts to a negative e^x below x ~= -5.61.
//
// Note: x is cosmosmath.LegacyDec; higher `terms` -> greater accuracy.
//
// A large positive x overflows the LegacyDec accumulator, which panics; that overflow is
// recovered and returned as an error instead. A large negative x underflows to zero, which
// is the closest LegacyDec representation of e^x in that regime.
func ExpDec(x math.LegacyDec, terms int) (result math.LegacyDec, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			result = math.LegacyDec{}
			err = fmt.Errorf("e^x overflow for x=%s: %v", x, rec)
		}
	}()

	one := math.LegacyOneDec()

	reduced := x
	squarings := 0
	for reduced.Abs().GT(one) {
		reduced = reduced.QuoInt64(2)
		squarings++
	}

	result = expSeries(reduced, terms)
	for i := 0; i < squarings; i++ {
		result = result.Mul(result)
	}

	if result.IsNegative() {
		return math.LegacyDec{}, fmt.Errorf("e^x invariant violated: negative result %s for x=%s", result, x)
	}
	if !x.IsPositive() && result.GT(one) {
		return math.LegacyDec{}, fmt.Errorf("e^x invariant violated: result %s exceeds 1 for non-positive x=%s", result, x)
	}

	return result, nil
}

// expSeries evaluates the Maclaurin expansion of e^x for the given term count.
// Accurate only for small |x|; callers must range-reduce x before calling it.
func expSeries(x math.LegacyDec, terms int) math.LegacyDec {
	result := math.LegacyOneDec()
	power := math.LegacyOneDec()
	factorial := math.LegacyOneDec()

	for i := 1; i <= terms; i++ {
		power = power.Mul(x)
		factorial = factorial.MulInt64(int64(i))
		result = result.Add(power.Quo(factorial))
	}

	return result
}
