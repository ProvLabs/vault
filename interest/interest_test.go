package interest_test

import (
	"math"
	"strconv"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/interest"
	"github.com/provlabs/vault/types"
	"github.com/stretchr/testify/require"
)

func TestCalculateInterestEarned(t *testing.T) {
	denom := "uatom"
	baseCoin := func(amt int64) sdk.Coin {
		return sdk.NewCoin(denom, sdkmath.NewInt(amt))
	}

	tests := []struct {
		name             string
		principal        sdk.Coin
		rate             string
		periodSeconds    int64
		expectedInterest sdkmath.Int
		expectedErrorMsg string
	}{
		{
			name:             "1 year at 0% APR",
			principal:        baseCoin(100_000_000),
			rate:             types.ZeroInterestRate,
			periodSeconds:    interest.SecondsPerYear,
			expectedInterest: sdkmath.NewInt(0),
		},
		{
			name:             "1 year at -100% APR",
			principal:        baseCoin(100_000_000),
			rate:             "-1.0",
			periodSeconds:    interest.SecondsPerYear,
			expectedInterest: sdkmath.NewInt(-63_212_055),
		},
		{
			name:             "1 year at 5% APR",
			principal:        baseCoin(100_000_000),
			rate:             "0.05",
			periodSeconds:    interest.SecondsPerYear,
			expectedInterest: sdkmath.NewInt(5_127_109),
		},
		{
			name:             "1 year at -5% APR",
			principal:        baseCoin(100_000_000),
			rate:             "-0.05",
			periodSeconds:    interest.SecondsPerYear,
			expectedInterest: sdkmath.NewInt(-4_877_057),
		},
		{
			name:             "6 months at 10% APR",
			principal:        baseCoin(500_000_000),
			rate:             "0.10",
			periodSeconds:    interest.SecondsPerYear / 2,
			expectedInterest: sdkmath.NewInt(25_635_548),
		},
		{
			name:             "zero period should error",
			principal:        baseCoin(100_000_000),
			rate:             "0.05",
			periodSeconds:    0,
			expectedErrorMsg: "periodSeconds must be positive",
		},
		{
			name:             "invalid rate string",
			principal:        baseCoin(100_000_000),
			rate:             "not_a_rate",
			periodSeconds:    interest.SecondsPerYear,
			expectedErrorMsg: "invalid rate string",
		},
		{
			name:             "tiny period, tiny rate",
			principal:        baseCoin(1_000_000),
			rate:             "0.00001",
			periodSeconds:    60,
			expectedInterest: sdkmath.NewInt(0),
		},
		{
			name:             "large amount over long period",
			principal:        baseCoin(1_000_000_000_000),
			rate:             "0.03",
			periodSeconds:    interest.SecondsPerYear * 10,
			expectedInterest: sdkmath.NewInt(349_858_807_576),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			interestAmt, err := interest.CalculateInterestEarned(tc.principal, tc.rate, tc.periodSeconds)

			if tc.expectedErrorMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErrorMsg)
				return
			}

			require.NoError(t, err)
			require.True(t, tc.expectedInterest.Equal(interestAmt), "interest amount doesn't match expected %s : %s", tc.expectedInterest.String(), interestAmt.String())
		})
	}
}

func TestCalculateExpiration(t *testing.T) {
	startTime := int64(1752764321)
	denom := "vault"

	tests := []struct {
		name           string
		principal      sdk.Coin
		reserves       sdk.Coin
		rate           string
		periodSeconds  int64
		startTime      int64
		limit          int64
		expected       int64
		expectedErrStr string
	}{
		{
			name:          "never expires with zero rate",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(100_000)),
			reserves:      sdk.NewCoin(denom, sdkmath.NewInt(500_000)),
			rate:          "0",
			periodSeconds: 60,
			startTime:     startTime,
			expected:      startTime,
		},
		{
			name:          "never expires with negative rate",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(100_000)),
			reserves:      sdk.NewCoin(denom, sdkmath.NewInt(500_000)),
			rate:          "-0.25",
			periodSeconds: 60,
			startTime:     startTime,
			expected:      startTime,
		},
		{
			name:          "never expires with zero principal",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(0)),
			reserves:      sdk.NewCoin(denom, sdkmath.NewInt(500_000)),
			rate:          "0.1",
			periodSeconds: 60,
			startTime:     startTime,
			expected:      startTime,
		},
		{
			name:          "depletes quickly with high rate",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(525_500_000)),
			reserves:      sdk.NewCoin(denom, sdkmath.NewInt(1_000)),
			rate:          "1.0",
			periodSeconds: 60,
			startTime:     startTime,
			expected:      startTime + 60,
		},
		{
			name:          "period limit interrupts expiration calculation",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(100)),
			reserves:      sdk.NewCoin(denom, sdkmath.NewInt(1_000_000)),
			rate:          "1.0",
			periodSeconds: interest.CalculatePeriodsLimit / 2,
			limit:         interest.CalculatePeriodsLimit,
			startTime:     startTime,
			expected:      startTime + interest.CalculatePeriodsLimit,
		},
		{
			name:          "never accrues when single period exceeds limit",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(100)),
			reserves:      sdk.NewCoin(denom, sdkmath.NewInt(1_000_000)),
			rate:          "1.0",
			periodSeconds: interest.CalculatePeriodsLimit + 1,
			limit:         interest.CalculatePeriodsLimit,
			startTime:     startTime,
			expected:      startTime,
		},
		{
			name:           "errors with denom mismatch",
			principal:      sdk.NewCoin("foo", sdkmath.NewInt(1000)),
			reserves:       sdk.NewCoin("bar", sdkmath.NewInt(1000)),
			rate:           "0.1",
			periodSeconds:  60,
			startTime:      startTime,
			expectedErrStr: "principal and vault denoms must match",
		},
		{
			name:           "errors with zero period",
			principal:      sdk.NewCoin(denom, sdkmath.NewInt(1000)),
			reserves:       sdk.NewCoin(denom, sdkmath.NewInt(1000)),
			rate:           "0.1",
			periodSeconds:  0,
			startTime:      startTime,
			expectedErrStr: "periodSeconds must be positive",
		},
		{
			name:           "errors with invalid rate",
			principal:      sdk.NewCoin(denom, sdkmath.NewInt(1000)),
			reserves:       sdk.NewCoin(denom, sdkmath.NewInt(1000)),
			rate:           "not-a-number",
			periodSeconds:  60,
			startTime:      startTime,
			expectedErrStr: "invalid rate string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limit := tc.limit
			if limit == 0 {
				limit = interest.CalculatePeriodsLimit
			}

			exp, err := interest.CalculateExpiration(tc.principal, tc.reserves, tc.rate, tc.periodSeconds, tc.startTime, limit)

			if tc.expectedErrStr != "" {
				require.Error(t, err, "expected error for test %q but got none", tc.name)
				require.Contains(t, err.Error(), tc.expectedErrStr,
					"expected error containing %q, got %q", tc.expectedErrStr, err.Error())
				return
			}

			require.NoError(t, err, "unexpected error for test %q: %v", tc.name, err)
			require.Equal(t, tc.expected, exp,
				"unexpected expiration time for test %q\nprincipal=%s reserves=%s rate=%s period=%d limit=%d",
				tc.name, tc.principal, tc.reserves, tc.rate, tc.periodSeconds, limit)
		})
	}
}

func TestCalculatePeriodsExtremes(t *testing.T) {
	t.Skip("Skipping test, used to explore edge cases")
	denom := "stake"

	tests := []struct {
		name          string
		principal     sdk.Coin
		vaultReserves sdk.Coin
		rate          string
		periodSeconds int64
		limit         int64
	}{
		{
			name:          "average values",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(1_000_000)),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewInt(500_000)),
			rate:          "0.05",
			periodSeconds: 86400, // 1 day
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "very high values, 1 day",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(1_000_000_000_000)),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewInt(10_000_000_000)),
			rate:          "0.08",
			periodSeconds: 86400,
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "very high values, 1 hour",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(1_000_000_000_000)),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewInt(10_000_000_000)),
			rate:          "0.08",
			periodSeconds: 3600, // 1 hour
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "very high values, 1 minute",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(1_000_000_000_000)),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewInt(10_000_000_000)),
			rate:          "0.08",
			periodSeconds: 60, // 1 minute
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "very high values, 5 seconds",
			principal:     sdk.NewCoin(denom, sdkmath.NewInt(1_000_000_000_000)),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewInt(10_000_000_000)),
			rate:          "0.08",
			periodSeconds: 5,
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "maximum values",
			principal:     sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			rate:          "1.0",
			periodSeconds: 86400,
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "maximum values, 1 hour",
			principal:     sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			rate:          "1.0",
			periodSeconds: 3600,
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "maximum values, 1 minute",
			principal:     sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			rate:          "1.0",
			periodSeconds: 60,
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "maximum values, 5 seconds",
			principal:     sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			rate:          "1.0",
			periodSeconds: 5,
			limit:         interest.CalculatePeriodsNoLimit,
		},
		{
			name:          "maximum values, 1 seconds",
			principal:     sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			vaultReserves: sdk.NewCoin(denom, sdkmath.NewIntFromUint64(^uint64(0))),
			rate:          "1.0",
			periodSeconds: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			periods, _, err := interest.CalculatePeriods(tc.vaultReserves, tc.principal, tc.rate, tc.periodSeconds, tc.limit)
			elapsed := time.Since(start)

			if err != nil {
				t.Errorf("error: %v", err)
				return
			}

			t.Logf("Test Case: %s", tc.name)
			t.Logf("Periods: %d", periods)
			t.Logf("Total Time: %.6f seconds", elapsed.Seconds())
			if periods > 0 {
				t.Logf("Time per period: %.6f ms", elapsed.Seconds()*1000/float64(periods))
			}
		})
	}
}

func TestExpDecInterestDrift(t *testing.T) {
	t.Skip("Skipping test, used to explore edge cases")
	principals := []int64{1_000, 10_000, 1_000_000, 100_000_000, 1_000_000_000, 10_000_000_000, 100_000_000_000, 1_000_000_000_000}
	durations := []int64{5, 3600, 7 * 24 * 3600} // 5 sec, 1 hr, 1 week
	rates := []string{"0.01", "0.05", "0.10", "0.25"}
	annualSeconds := 31_536_000

	for _, rate := range rates {
		rateF, err := strconv.ParseFloat(rate, 64)
		require.NoError(t, err)

		for _, principalAmt := range principals {
			for _, duration := range durations {
				principal := sdk.NewCoin("test", sdkmath.NewInt(principalAmt))

				earned, err := interest.CalculateInterestEarned(principal, rate, duration)
				require.NoError(t, err)
				sdkInterest := earned.Int64()

				tYears := float64(duration) / float64(annualSeconds)
				expected := float64(principalAmt) * (math.Exp(rateF*tYears) - 1)
				stdInterest := int64(expected)

				diff := sdkInterest - stdInterest
				percentDrift := 100 * float64(diff) / float64(principalAmt)

				t.Logf("terms=%d, rate=%s, principal=%d, duration=%ds → sdk=%d, std=%d, drift=%d (%.10f%%)",
					interest.EulerPrecision, rate, principalAmt, duration, sdkInterest, stdInterest, diff, percentDrift)
			}
		}
	}
}

