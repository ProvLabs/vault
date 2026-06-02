package utils

import (
	"fmt"

	"cosmossdk.io/math"
)

// ExpDec calculates e^x using the Maclaurin series expansion up to `terms` terms.
// Fully deterministic and safe for on-chain use.
//
//	e^x = 1 + x + x^2/2! + x^3/3! + ... + x^n/n!
//
// Note: x is cosmosmath.LegacyDec; higher `terms` -> greater accuracy.
//
// A large |x| overflows the LegacyDec accumulator, which panics; that overflow is
// recovered and returned as an error instead. Callers should still bound x upstream
// (see types.MaxAbsInterestRate); this guard is defense-in-depth.
func ExpDec(x math.LegacyDec, terms int) (result math.LegacyDec, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			result = math.LegacyDec{}
			err = fmt.Errorf("e^x overflow for x=%s: %v", x, rec)
		}
	}()

	result = math.LegacyOneDec()
	power := math.LegacyOneDec()
	factorial := math.LegacyOneDec()

	for i := 1; i <= terms; i++ {
		power = power.Mul(x)
		factorial = factorial.MulInt64(int64(i))
		term := power.Quo(factorial)
		result = result.Add(term)
	}

	return result, nil
}
