package utils_test

import (
	"math"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/provlabs/vault/utils"
	"github.com/stretchr/testify/require"
)

func TestExpDec(t *testing.T) {
	tests := []struct {
		name      string
		input     sdkmath.LegacyDec
		terms     int
		expected  float64
		tolerance float64
	}{
		{
			name:      "e^0 = 1",
			input:     sdkmath.LegacyZeroDec(),
			terms:     17,
			expected:  1.0,
			tolerance: 1e-17,
		},
		{
			name:      "e^1 ~= 2.71828",
			input:     sdkmath.LegacyNewDec(1),
			terms:     17,
			expected:  math.E,
			tolerance: 1e-17,
		},
		{
			name:      "e^-1 ~= 0.36788",
			input:     sdkmath.LegacyNewDec(-1),
			terms:     17,
			expected:  math.Exp(-1),
			tolerance: 1e-6,
		},
		{
			name:      "e^2 ~= 7.38906",
			input:     sdkmath.LegacyNewDec(2),
			terms:     17,
			expected:  math.Exp(2),
			tolerance: 1e-5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := utils.ExpDec(tc.input, tc.terms)
			got, _ := result.Float64()
			diff := math.Abs(got - tc.expected)
			require.LessOrEqual(t, diff, tc.tolerance, "expected %f, got %f", tc.expected, got)
		})
	}
}

func TestExpDecConvergenceToE(t *testing.T) {
	t.Skip("Skipping test, used to explore edge cases")
	target := math.E
	tolerance := 1e-17
	maxTerms := 100
	x := sdkmath.LegacyNewDec(1)

	var closest float64
	var matched bool
	var termsUsed int

	for terms := 1; terms <= maxTerms; terms++ {
		start := time.Now()

		result := utils.ExpDec(x, terms)

		elapsed := time.Since(start).Microseconds()

		f, err := result.Float64()
		require.NoError(t, err)

		diff := math.Abs(f - target)
		t.Logf("terms=%d took %d µs, result=%.20f, diff=%.20f", terms, elapsed, f, diff)

		if diff < tolerance {
			matched = true
			closest = f
			termsUsed = terms
			break
		}
	}

	require.True(t, matched, "did not converge to math.E within %g tolerance", tolerance)
	t.Logf("Matched math.E (%.20f) with %d terms: %.20f", target, termsUsed, closest)
}

