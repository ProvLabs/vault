package utils

import (
	"cosmossdk.io/math"
)

// ExpDec calculates e^x using the Maclaurin series expansion up to `terms` terms.
// Safe for on-chain use (fully deterministic).
//
//	e^x = 1 + x + x^2/2! + x^3/3! + ... + x^n/n!
//
// Note: x is cosmosmath.LegacyDec; higher `terms` -> greater accuracy.
func ExpDec(x math.LegacyDec, terms int) math.LegacyDec {
	result := math.LegacyOneDec()
	power := math.LegacyOneDec()
	factorial := math.LegacyOneDec()

	for i := 1; i <= terms; i++ {
		power = power.Mul(x)
		factorial = factorial.MulInt64(int64(i))
		term := power.Quo(factorial)
		result = result.Add(term)
	}

	return result
}
