package utils_test

import (
	"fmt"
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

func TestExpDecInterestError(t *testing.T) {
	// t.Skip("exploratory")

	rates := []float64{0.01, 0.05, 0.20, 0.50, 1.0}
	years := []float64{1.0 / 365.0, 30.0 / 365.0, 1.0, 5.0, 10.0}
	terms := 17

	denom := "uusd"
	principalAmt := int64(1_000_000_000_000_000)
	principal := sdkmath.NewInt(principalAmt)
	pDec := sdkmath.LegacyNewDecFromInt(principal)

	for _, r := range rates {
		for _, y := range years {
			rt := r * y

			trueF64 := math.Exp(rt)

			rtStr := fmt.Sprintf("%.18f", rt)
			rtDec, err := sdkmath.LegacyNewDecFromStr(rtStr)
			if err != nil {
				t.Fatalf("rt=%f LegacyNewDecFromStr err: %v", rt, err)
			}

			approxDec := utils.ExpDec(rtDec, terms)
			approxF64, err := approxDec.Float64()
			if err != nil {
				t.Fatalf("approx Float64 err: %v", err)
			}

			relErr := math.Abs(approxF64-trueF64) / trueF64

			trueExpDecStr := fmt.Sprintf("%.18f", trueF64)
			trueExpDec, err := sdkmath.LegacyNewDecFromStr(trueExpDecStr)
			if err != nil {
				t.Fatalf("trueExpDec err: %v", err)
			}

			trueFinal := pDec.Mul(trueExpDec)
			approxFinal := pDec.Mul(approxDec)

			trueInterest := trueFinal.Sub(pDec)
			approxInterest := approxFinal.Sub(pDec)

			intErr := approxInterest.Sub(trueInterest).Abs()
			intErrInt := intErr.TruncateInt()

			fmt.Printf("r=%.2f t=%.4f years rt=%.6f relErr(e^(rt))=%.3e interestError=%s %s\n",
				r, y, rt, relErr, intErrInt.String(), denom)
		}
	}
}

func TestExpDecInterestErrorSweep(t *testing.T) {
	// t.Skip("exploratory")

	terms := 17

	denom := "uusd"
	principalAmt := int64(1_000_000_000_000_000)
	principal := sdkmath.NewInt(principalAmt)
	pDec := sdkmath.LegacyNewDecFromInt(principal)

	startX := 0.0
	step := 0.001
	maxX := 20.0

	for x := startX; x <= maxX; x += step {
		trueF64 := math.Exp(x)

		xStr := fmt.Sprintf("%.18f", x)
		xDec, err := sdkmath.LegacyNewDecFromStr(xStr)
		if err != nil {
			t.Fatalf("x=%f LegacyNewDecFromStr err: %v", x, err)
		}

		approxDec := utils.ExpDec(xDec, terms)
		approxF64, err := approxDec.Float64()
		if err != nil {
			t.Fatalf("approx Float64 err: %v", err)
		}

		relErr := math.Abs(approxF64-trueF64) / trueF64

		trueExpDecStr := fmt.Sprintf("%.18f", trueF64)
		trueExpDec, err := sdkmath.LegacyNewDecFromStr(trueExpDecStr)
		if err != nil {
			t.Fatalf("trueExpDec err: %v", err)
		}

		trueFinal := pDec.Mul(trueExpDec)
		approxFinal := pDec.Mul(approxDec)

		trueInterest := trueFinal.Sub(pDec)
		approxInterest := approxFinal.Sub(pDec)

		intErr := approxInterest.Sub(trueInterest).Abs()
		intErrInt := intErr.TruncateInt()

		if !intErrInt.IsZero() {
			fmt.Printf("x=%.6f relErr(e^x)=%.3e interestError=%s %s\n",
				x, relErr, intErrInt.String(), denom)
			break
		}
	}
}
