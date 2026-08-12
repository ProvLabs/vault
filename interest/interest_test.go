package interest_test

import (
	"math"
	"math/big"
	"strconv"
	"testing"

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
		{
			name:             "extreme rate returns overflow error instead of panicking",
			principal:        baseCoin(100_000_000),
			rate:             "1000000000000.0",
			periodSeconds:    interest.SecondsPerYear,
			expectedErrorMsg: "overflow",
		},
		{
			name:             "22 days at the max negative rate reclaims under the full principal",
			principal:        baseCoin(1_000_000_000),
			rate:             "-" + types.MaxAbsInterestRate,
			periodSeconds:    22 * interest.SecondsPerDay,
			expectedInterest: sdkmath.NewInt(-997_588_236),
		},
		{
			name:             "10 years at the max positive rate errors instead of silently under-paying",
			principal:        baseCoin(1),
			rate:             types.MaxAbsInterestRate,
			periodSeconds:    interest.SecondsPerYear * 10,
			expectedErrorMsg: "overflow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			interestAmt, err := interest.CalculateInterestEarned(tc.principal, tc.rate, tc.periodSeconds)

			if tc.expectedErrorMsg != "" {
				require.Errorf(t, err, "test case %q: expected error but got none", tc.name)
				require.Containsf(t, err.Error(), tc.expectedErrorMsg, "test case %q: error message mismatch", tc.name)
				return
			}

			require.NoErrorf(t, err, "test case %q: unexpected error", tc.name)
			require.Truef(t, tc.expectedInterest.Equal(interestAmt), "test case %q: interest amount doesn't match; expected %s, got %s", tc.name, tc.expectedInterest.String(), interestAmt.String())
		})
	}
}

func TestCalculateInterestEarnedNeverReclaimsMoreThanPrincipal(t *testing.T) {
	principal := sdk.NewCoin("uatom", sdkmath.NewInt(1_000_000_000))

	tests := []struct {
		name          string
		rate          string
		periodSeconds int64
	}{
		{name: "max negative rate just under the old sign crossover", rate: "-" + types.MaxAbsInterestRate, periodSeconds: 20 * interest.SecondsPerDay},
		{name: "max negative rate at the old sign crossover", rate: "-" + types.MaxAbsInterestRate, periodSeconds: 21 * interest.SecondsPerDay},
		{name: "max negative rate past the old sign crossover", rate: "-" + types.MaxAbsInterestRate, periodSeconds: 22 * interest.SecondsPerDay},
		{name: "max negative rate over a full month", rate: "-" + types.MaxAbsInterestRate, periodSeconds: 30 * interest.SecondsPerDay},
		{name: "max negative rate over a full year", rate: "-" + types.MaxAbsInterestRate, periodSeconds: interest.SecondsPerYear},
		{name: "moderate negative rate over a decade", rate: "-1.0", periodSeconds: interest.SecondsPerYear * 10},
		{name: "small negative rate over a century", rate: "-0.1", periodSeconds: interest.SecondsPerYear * 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			interestAmt, err := interest.CalculateInterestEarned(principal, tc.rate, tc.periodSeconds)
			require.NoErrorf(t, err, "test case %q: unexpected error for rate=%s over %d seconds", tc.name, tc.rate, tc.periodSeconds)
			require.Truef(t, interestAmt.IsNegative(), "test case %q: a negative rate must produce negative interest, got %s", tc.name, interestAmt)
			require.Truef(t, interestAmt.Abs().LTE(principal.Amount), "test case %q: reclaim %s exceeds the %s principal, implying more than 100%% of the vault's underlying", tc.name, interestAmt.Abs(), principal.Amount)
		})
	}
}

func TestCalculateAUMFee(t *testing.T) {
	tests := []struct {
		name                string
		aum                 sdkmath.Int
		bips                uint32
		duration            int64
		expectedFee         sdkmath.Int
		expectErr           bool
		expectedErrContains string
	}{
		{
			name:        "zero AUM",
			aum:         sdkmath.ZeroInt(),
			bips:        15,
			duration:    interest.SecondsPerYear,
			expectedFee: sdkmath.ZeroInt(),
		},
		{
			name:        "zero duration",
			aum:         sdkmath.NewInt(1_000_000),
			bips:        15,
			duration:    0,
			expectedFee: sdkmath.ZeroInt(),
		},
		{
			name:        "zero bips",
			aum:         sdkmath.NewInt(1_000_000),
			bips:        0,
			duration:    interest.SecondsPerYear,
			expectedFee: sdkmath.ZeroInt(),
		},
		{
			name:        "1 year at 15 bps (1,000,000 AUM)",
			aum:         sdkmath.NewInt(1_000_000),
			bips:        15,
			duration:    interest.SecondsPerYear,
			expectedFee: sdkmath.NewInt(1_500), // 1,000,000 * 0.0015
		},
		{
			name:        "6 months at 15 bps (1,000,000 AUM)",
			aum:         sdkmath.NewInt(1_000_000),
			bips:        15,
			duration:    interest.SecondsPerYear / 2,
			expectedFee: sdkmath.NewInt(750),
		},
		{
			name:                "negative duration errors",
			aum:                 sdkmath.NewInt(1_000_000),
			bips:                15,
			duration:            -1,
			expectErr:           true,
			expectedErrContains: "duration cannot be negative",
		},
		{
			name:                "negative aum errors",
			aum:                 sdkmath.NewInt(-1_000_000),
			bips:                15,
			duration:            interest.SecondsPerYear,
			expectErr:           true,
			expectedErrContains: "aum cannot be negative",
		},
		{
			name:                "near-max AUM over a one year period overflows the decimal multiply",
			aum:                 sdkmath.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 255)),
			bips:                10_000,
			duration:            interest.SecondsPerYear,
			expectErr:           true,
			expectedErrContains: "overflow",
		},
		{
			name:                "max AUM over a single second overflows the decimal multiply",
			aum:                 sdkmath.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))),
			bips:                10_000,
			duration:            2,
			expectErr:           true,
			expectedErrContains: "overflow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fee, err := interest.CalculateAUMFee(tc.aum, tc.bips, tc.duration)
			if tc.expectErr {
				require.Errorf(t, err, "test case %q: expected an error but got none", tc.name)
				require.Containsf(t, err.Error(), tc.expectedErrContains, "test case %q: error %q should contain %q", tc.name, err, tc.expectedErrContains)
				require.Truef(t, fee.IsNil(), "test case %q: fee should be the zero-value sdkmath.Int on error, got %s", tc.name, fee)
			} else {
				require.NoErrorf(t, err, "test case %q: unexpected error during AUM fee calculation", tc.name)
				require.Truef(t, tc.expectedFee.Equal(fee), "test case %q: fee amount mismatch; expected %s, got %s", tc.name, tc.expectedFee, fee)
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
		require.NoErrorf(t, err, "failed to parse rate %q", rate)

		for _, principalAmt := range principals {
			for _, duration := range durations {
				principal := sdk.NewCoin("test", sdkmath.NewInt(principalAmt))

				earned, err := interest.CalculateInterestEarned(principal, rate, duration)
				require.NoErrorf(t, err, "CalculateInterestEarned failed for rate=%s, principal=%d, duration=%d", rate, principalAmt, duration)
				sdkInterest := earned.Int64()

				tYears := float64(duration) / float64(annualSeconds)
				expected := float64(principalAmt) * (math.Exp(rateF*tYears) - 1)
				stdInterest := int64(expected)

				diff := sdkInterest - stdInterest
				percentDrift := 100 * float64(diff) / float64(principalAmt)

				t.Logf("terms=%d, rate=%s, principal=%d, duration=%ds \u2192 sdk=%d, std=%d, drift=%d (%.10f%%)",
					interest.EulerPrecision, rate, principalAmt, duration, sdkInterest, stdInterest, diff, percentDrift)
			}
		}
	}
}
