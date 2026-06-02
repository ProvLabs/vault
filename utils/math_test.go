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
			result, err := utils.ExpDec(tc.input, tc.terms)
			require.NoError(t, err, "ExpDec returned an unexpected error")
			got, _ := result.Float64()
			diff := math.Abs(got - tc.expected)
			require.LessOrEqual(t, diff, tc.tolerance, "expected %f, got %f", tc.expected, got)
		})
	}
}

func TestExpDecOverflow(t *testing.T) {
	tests := []struct {
		name    string
		input   sdkmath.LegacyDec
		terms   int
		wantErr bool
	}{
		{
			name:    "large positive exponent overflows and returns error",
			input:   sdkmath.LegacyNewDec(1_000_000),
			terms:   17,
			wantErr: true,
		},
		{
			name:    "large negative exponent overflows and returns error",
			input:   sdkmath.LegacyNewDec(-1_000_000),
			terms:   17,
			wantErr: true,
		},
		{
			name:    "moderate exponent does not overflow",
			input:   sdkmath.LegacyNewDec(10),
			terms:   17,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			require.NotPanics(t, func() {
				_, err = utils.ExpDec(tc.input, tc.terms)
			}, "ExpDec must never panic, even on overflow, for input %s", tc.input)

			if tc.wantErr {
				require.ErrorContains(t, err, "overflow", "expected an overflow error for input %s", tc.input)
			} else {
				require.NoError(t, err, "did not expect an error for input %s", tc.input)
			}
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

		result, expErr := utils.ExpDec(x, terms)
		require.NoErrorf(t, expErr, "ExpDec failed for terms=%d", terms)

		elapsed := time.Since(start).Microseconds()

		f, err := result.Float64()
		require.NoErrorf(t, err, "Float64 conversion failed for terms=%d", terms)

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
