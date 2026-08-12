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
		name         string
		input        sdkmath.LegacyDec
		terms        int
		expected     float64
		relTolerance float64
	}{
		{
			name:         "e^0 = 1",
			input:        sdkmath.LegacyZeroDec(),
			terms:        17,
			expected:     1.0,
			relTolerance: 1e-17,
		},
		{
			name:         "e^1 ~= 2.71828",
			input:        sdkmath.LegacyNewDec(1),
			terms:        17,
			expected:     math.E,
			relTolerance: 1e-15,
		},
		{
			name:         "e^-1 ~= 0.36788",
			input:        sdkmath.LegacyNewDec(-1),
			terms:        17,
			expected:     math.Exp(-1),
			relTolerance: 1e-15,
		},
		{
			name:         "e^2 requires one range-reduction halving",
			input:        sdkmath.LegacyNewDec(2),
			terms:        17,
			expected:     math.Exp(2),
			relTolerance: 1e-15,
		},
		{
			name:         "e^10 no longer under-shoots by 1.4 percent",
			input:        sdkmath.LegacyNewDec(10),
			terms:        17,
			expected:     math.Exp(10),
			relTolerance: 1e-15,
		},
		{
			name:         "e^15 no longer under-shoots by 25 percent",
			input:        sdkmath.LegacyNewDec(15),
			terms:        17,
			expected:     math.Exp(15),
			relTolerance: 1e-15,
		},
		{
			name:         "e^20 no longer under-shoots by 70 percent",
			input:        sdkmath.LegacyNewDec(20),
			terms:        17,
			expected:     math.Exp(20),
			relTolerance: 1e-15,
		},
		{
			name:         "e^30 stays accurate deep into the positive range",
			input:        sdkmath.LegacyNewDec(30),
			terms:        17,
			expected:     math.Exp(30),
			relTolerance: 1e-15,
		},
		{
			name:         "e^-5 is accurate below the old drift onset",
			input:        sdkmath.LegacyNewDec(-5),
			terms:        17,
			expected:     math.Exp(-5),
			relTolerance: 1e-15,
		},
		{
			name:         "e^-5.62 is positive past the old sign crossover",
			input:        sdkmath.LegacyMustNewDecFromStr("-5.62"),
			terms:        17,
			expected:     math.Exp(-5.62),
			relTolerance: 1e-15,
		},
		{
			name:         "e^-6 is positive where the series previously inverted",
			input:        sdkmath.LegacyNewDec(-6),
			terms:        17,
			expected:     math.Exp(-6),
			relTolerance: 1e-15,
		},
		{
			name:         "e^-10 is positive where the series previously returned -101.69",
			input:        sdkmath.LegacyNewDec(-10),
			terms:        17,
			expected:     math.Exp(-10),
			relTolerance: 1e-14,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := utils.ExpDec(tc.input, tc.terms)
			require.NoError(t, err, "ExpDec returned an unexpected error for x=%s", tc.input)
			got, _ := result.Float64()
			relDiff := math.Abs(got-tc.expected) / math.Abs(tc.expected)
			require.LessOrEqual(t, relDiff, tc.relTolerance, "e^%s: expected %g, got %g (relative error %g)", tc.input, tc.expected, got, relDiff)
		})
	}
}

func TestExpDecExtremeExponents(t *testing.T) {
	tests := []struct {
		name           string
		input          sdkmath.LegacyDec
		terms          int
		wantErr        bool
		expectedResult sdkmath.LegacyDec
	}{
		{
			name:    "large positive exponent overflows and returns error",
			input:   sdkmath.LegacyNewDec(1_000_000),
			terms:   17,
			wantErr: true,
		},
		{
			name:           "large negative exponent underflows to zero without error",
			input:          sdkmath.LegacyNewDec(-1_000_000),
			terms:          17,
			expectedResult: sdkmath.LegacyZeroDec(),
		},
		{
			name:           "exponent at the LegacyDec representable ceiling stays finite",
			input:          sdkmath.LegacyNewDec(136),
			terms:          17,
			expectedResult: sdkmath.LegacyMustNewDecFromStr("115890954241388550625166098655921281262367170759088580448088.610547201010098483"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var result sdkmath.LegacyDec
			var err error
			require.NotPanics(t, func() {
				result, err = utils.ExpDec(tc.input, tc.terms)
			}, "ExpDec must never panic, even on overflow, for input %s", tc.input)

			if tc.wantErr {
				require.ErrorContains(t, err, "overflow", "expected an overflow error for input %s", tc.input)
				return
			}

			require.NoError(t, err, "did not expect an error for input %s", tc.input)
			require.True(t, tc.expectedResult.Equal(result), "e^%s: expected %s, got %s", tc.input, tc.expectedResult, result)
		})
	}
}

func TestExpDecInvariants(t *testing.T) {
	exponents := []string{
		"-1000", "-100", "-40", "-20", "-10", "-8", "-6", "-5.6112", "-5.61", "-5", "-2.5", "-1", "-0.000000000000000001",
		"0",
		"0.000000000000000001", "1", "2.5", "5", "10", "15", "20", "30", "60", "100", "136",
	}
	one := sdkmath.LegacyOneDec()

	for _, exponent := range exponents {
		t.Run(exponent, func(t *testing.T) {
			x := sdkmath.LegacyMustNewDecFromStr(exponent)
			result, err := utils.ExpDec(x, 17)
			require.NoError(t, err, "ExpDec must not error inside the representable range for x=%s", exponent)
			require.False(t, result.IsNegative(), "e^%s must never be negative, got %s", exponent, result)

			if x.IsNegative() {
				require.True(t, result.LTE(one), "e^%s must not exceed 1 for negative x, got %s", exponent, result)
			} else {
				require.True(t, result.GTE(one), "e^%s must be at least 1 for non-negative x, got %s", exponent, result)
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
