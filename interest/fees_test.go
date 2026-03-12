package interest_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/provlabs/vault/interest"
	"github.com/stretchr/testify/require"
)

func TestCalculateAUMFee(t *testing.T) {
	tests := []struct {
		name        string
		aum         sdkmath.Int
		duration    int64
		expectedFee sdkmath.Int
	}{
		{
			name:        "zero AUM",
			aum:         sdkmath.ZeroInt(),
			duration:    interest.SecondsPerYear,
			expectedFee: sdkmath.ZeroInt(),
		},
		{
			name:        "zero duration",
			aum:         sdkmath.NewInt(1_000_000_000),
			duration:    0,
			expectedFee: sdkmath.ZeroInt(),
		},
		{
			name:        "negative duration",
			aum:         sdkmath.NewInt(1_000_000_000),
			duration:    -100,
			expectedFee: sdkmath.ZeroInt(),
		},
		{
			name:        "1 year at 15 bps (100M AUM)",
			aum:         sdkmath.NewInt(100_000_000),
			duration:    interest.SecondsPerYear,
			expectedFee: sdkmath.NewInt(150_000), // 100,000,000 * 0.0015 = 150,000
		},
		{
			name:        "1 day at 15 bps (100M AUM)",
			aum:         sdkmath.NewInt(100_000_000),
			duration:    interest.SecondsPerDay,
			expectedFee: sdkmath.NewInt(410), // (100,000,000 * 0.0015 * 86,400) / 31,536,000 = 410.95... -> 410 (truncated)
		},
		{
			name:        "1 hour at 15 bps (1B AUM)",
			aum:         sdkmath.NewInt(1_000_000_000),
			duration:    interest.SecondsPerHour,
			expectedFee: sdkmath.NewInt(171), // (1,000,000,000 * 0.0015 * 3,600) / 31,536,000 = 171.23... -> 171
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fee, err := interest.CalculateAUMFee(tc.aum, tc.duration)
			require.NoError(t, err, "failed to calculate AUM fee for AUM %s, duration %d", tc.aum, tc.duration)
			require.True(t, tc.expectedFee.Equal(fee), "AUM fee calculation mismatch: expected %s, got %s (AUM: %s, duration: %d)", tc.expectedFee, fee, tc.aum, tc.duration)
		})
	}
}
