package interest_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/interest"
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
		expectedMin      sdk.Coin
		expectedInterest sdk.Coin
		expectErr        bool
		expectedErrorMsg string
	}{
		{
			name:             "1 year at 5% APR",
			principal:        baseCoin(100_000_000),
			rate:             "0.05",
			periodSeconds:    interest.SecondsPerYear, // 1 year
			expectedInterest: baseCoin(5_127_109),
			expectErr:        false,
		},
		{
			name:             "6 months at 10% APR",
			principal:        baseCoin(500_000_000),
			rate:             "0.10",
			periodSeconds:    interest.SecondsPerYear / 2,
			expectedInterest: baseCoin(25_635_548),
			expectErr:        false,
		},
		{
			name:             "zero period should error",
			principal:        baseCoin(100_000_000),
			rate:             "0.05",
			periodSeconds:    0,
			expectErr:        true,
			expectedErrorMsg: "periodSeconds must be positive",
		},
		{
			name:             "invalid rate string",
			principal:        baseCoin(100_000_000),
			rate:             "not_a_rate",
			periodSeconds:    interest.SecondsPerYear,
			expectErr:        true,
			expectedErrorMsg: "invalid rate string",
		},
		{
			name:             "tiny period, tiny rate",
			principal:        baseCoin(1_000_000),
			rate:             "0.00001",
			periodSeconds:    60, // 1 minute
			expectedInterest: baseCoin(0),
			expectErr:        false,
		},
		{
			name:             "large amount over long period",
			principal:        baseCoin(1_000_000_000_000),
			rate:             "0.03",
			periodSeconds:    interest.SecondsPerYear * 10, // 10 years
			expectedInterest: baseCoin(349_858_807_576),
			expectErr:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			interest, err := interest.CalculateInterestEarned(tc.principal, tc.rate, tc.periodSeconds)

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErrorMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.principal.Denom, interest.Denom)

			require.True(t, interest.Amount.Equal(tc.expectedInterest.Amount), fmt.Sprintf("got %s, expected <= %s", interest.Amount, tc.expectedInterest.Amount))
		})
	}
}

func TestCalculateExpiration(t *testing.T) {
	require.Fail(t, "not implemented")
}
