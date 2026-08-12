package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provenance-io/provenance/x/exchange"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/provlabs/vault/types"
)

func TestNewPaymentFromExchange(t *testing.T) {
	source := NewTestAddress()
	target := NewTestAddress()
	sourceAmount := sdk.NewCoins(sdk.NewInt64Coin("rwacoin", 10))
	targetAmount := sdk.NewCoins(sdk.NewInt64Coin("under", 5))

	converted := types.NewPaymentFromExchange(&exchange.Payment{
		Source:       source,
		SourceAmount: sourceAmount,
		Target:       target,
		TargetAmount: targetAmount,
		ExternalId:   "payment-1",
	})

	assert.Equal(t, types.Payment{
		Source:       source,
		SourceAmount: sourceAmount,
		Target:       target,
		TargetAmount: targetAmount,
		ExternalId:   "payment-1",
	}, converted, "converted payment should carry every field of the exchange payment")
}

func TestPayment_ValidateMatches(t *testing.T) {
	source := NewTestAddress()
	vaultAddr := NewTestAddress()
	otherAddr := NewTestAddress()

	approved := types.Payment{
		Source:       source,
		SourceAmount: sdk.NewCoins(sdk.NewInt64Coin("rwacoin", 2)),
		Target:       vaultAddr,
		TargetAmount: sdk.NewCoins(sdk.NewInt64Coin("under", 1)),
		ExternalId:   "payment-1",
	}

	storedFrom := func(mutate func(*exchange.Payment)) *exchange.Payment {
		stored := &exchange.Payment{
			Source:       approved.Source,
			SourceAmount: approved.SourceAmount,
			Target:       approved.Target,
			TargetAmount: approved.TargetAmount,
			ExternalId:   approved.ExternalId,
		}
		if mutate != nil {
			mutate(stored)
		}
		return stored
	}

	tests := []struct {
		name        string
		stored      *exchange.Payment
		expectedErr string
	}{
		{
			name:   "stored payment still carries the approved terms",
			stored: storedFrom(nil),
		},
		{
			name:        "no stored payment",
			stored:      nil,
			expectedErr: "no stored payment to match against",
		},
		{
			name:        "source replaced",
			stored:      storedFrom(func(p *exchange.Payment) { p.Source = otherAddr }),
			expectedErr: "approved source",
		},
		{
			name:        "external id replaced",
			stored:      storedFrom(func(p *exchange.Payment) { p.ExternalId = "payment-2" }),
			expectedErr: "approved external id",
		},
		{
			name:        "retargeted away from the approved vault",
			stored:      storedFrom(func(p *exchange.Payment) { p.Target = otherAddr }),
			expectedErr: "approved target",
		},
		{
			name: "source leg enlarged",
			stored: storedFrom(func(p *exchange.Payment) {
				p.SourceAmount = sdk.NewCoins(sdk.NewInt64Coin("rwacoin", 100))
			}),
			expectedErr: "approved source amount",
		},
		{
			name: "source leg denom swapped",
			stored: storedFrom(func(p *exchange.Payment) {
				p.SourceAmount = sdk.NewCoins(sdk.NewInt64Coin("otherasset", 2))
			}),
			expectedErr: "approved source amount",
		},
		{
			name: "target leg enlarged",
			stored: storedFrom(func(p *exchange.Payment) {
				p.TargetAmount = sdk.NewCoins(sdk.NewInt64Coin("under", 50))
			}),
			expectedErr: "approved target amount",
		},
		{
			name: "legs swapped to flip the settlement direction",
			stored: storedFrom(func(p *exchange.Payment) {
				p.SourceAmount, p.TargetAmount = approved.TargetAmount, approved.SourceAmount
			}),
			expectedErr: "approved source amount",
		},
		{
			name: "target leg emptied",
			stored: storedFrom(func(p *exchange.Payment) {
				p.TargetAmount = sdk.NewCoins()
			}),
			expectedErr: "approved target amount",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := approved.ValidateMatches(tc.stored)
			if tc.expectedErr == "" {
				assert.NoError(t, err, "matching terms should be accepted for case %q", tc.name)
				return
			}
			require.Error(t, err, "substituted terms must be rejected for case %q", tc.name)
			assert.Contains(t, err.Error(), tc.expectedErr, "error should name the substituted field for case %q", tc.name)
		})
	}
}

func TestPayment_ValidateMatches_TreatsEmptyAndNilLegsAsEqual(t *testing.T) {
	source := NewTestAddress()
	vaultAddr := NewTestAddress()

	approved := types.Payment{
		Source:       source,
		SourceAmount: sdk.NewCoins(sdk.NewInt64Coin("rwacoin", 2)),
		Target:       vaultAddr,
		TargetAmount: sdk.NewCoins(),
		ExternalId:   "zero-priced",
	}
	stored := &exchange.Payment{
		Source:       source,
		SourceAmount: sdk.NewCoins(sdk.NewInt64Coin("rwacoin", 2)),
		Target:       vaultAddr,
		TargetAmount: nil,
		ExternalId:   "zero-priced",
	}

	assert.NoError(t, approved.ValidateMatches(stored), "a zero-priced leg should match whether it is stored as nil or as empty coins")
}
