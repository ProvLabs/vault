package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provlabs/vault/types"
)

func (s *TestSuite) TestHandleVaultLiquidity_Table() {
	underlyingDenom := "nhash"
	paymentDenom := "usdc"
	shareDenom := "vshare"
	otherDenom := "other"

	type testCase struct {
		name             string
		pendingSwapOuts  []types.PendingSwapOut
		balances         sdk.Coins
		payments         []types.Payment
		expectedAccepted []uint64
	}

	cases := []testCase{
		{
			name:            "no pending swap-outs - no action",
			pendingSwapOuts: []types.PendingSwapOut{},
			balances:        sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 0)),
			payments: []types.Payment{
				{
					ID:           1,
					Source:       "source1",
					SourceAmount: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100)),
				},
			},
			expectedAccepted: []uint64{},
		},
		{
			name: "pending swap-out for payment denom, zero liquidity - accepts payment",
			pendingSwapOuts: []types.PendingSwapOut{
				{
					Owner:       s.adminAddr.String(),
					Shares:      sdk.NewInt64Coin(shareDenom, 10),
					RedeemDenom: paymentDenom,
				},
			},
			balances: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 0)),
			payments: []types.Payment{
				{
					ID:           1,
					Source:       "source1",
					SourceAmount: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100)),
				},
			},
			expectedAccepted: []uint64{1},
		},
		{
			name: "pending swap-out for underlying asset, zero liquidity - accepts payment providing underlying",
			pendingSwapOuts: []types.PendingSwapOut{
				{
					Owner:       s.adminAddr.String(),
					Shares:      sdk.NewInt64Coin(shareDenom, 10),
					RedeemDenom: underlyingDenom,
				},
			},
			balances: sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 0), sdk.NewInt64Coin(paymentDenom, 100)),
			payments: []types.Payment{
				{
					ID:           1,
					Source:       "source1",
					SourceAmount: sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100)),
				},
			},
			expectedAccepted: []uint64{1},
		},
		{
			name: "default to payment denom - accepts payment even if only underlying requested",
			pendingSwapOuts: []types.PendingSwapOut{
				{
					Owner:       s.adminAddr.String(),
					Shares:      sdk.NewInt64Coin(shareDenom, 10),
					RedeemDenom: underlyingDenom,
				},
			},
			balances: sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100), sdk.NewInt64Coin(paymentDenom, 0)),
			payments: []types.Payment{
				{
					ID:           1,
					Source:       "source1",
					SourceAmount: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100)),
				},
			},
			expectedAccepted: []uint64{1},
		},
		{
			name: "liquidity sufficient - no action",
			pendingSwapOuts: []types.PendingSwapOut{
				{
					Owner:       s.adminAddr.String(),
					Shares:      sdk.NewInt64Coin(shareDenom, 10),
					RedeemDenom: paymentDenom,
				},
			},
			balances: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100)),
			payments: []types.Payment{
				{
					ID:           1,
					Source:       "source1",
					SourceAmount: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100)),
				},
			},
			expectedAccepted: []uint64{},
		},
		{
			name: "multiple needed denoms - accepts multiple sources",
			pendingSwapOuts: []types.PendingSwapOut{
				{
					Owner:       s.adminAddr.String(),
					Shares:      sdk.NewInt64Coin(shareDenom, 10),
					RedeemDenom: underlyingDenom,
				},
				{
					Owner:       s.adminAddr.String(),
					Shares:      sdk.NewInt64Coin(shareDenom, 10),
					RedeemDenom: otherDenom, 
				},
			},
			balances: sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 0), sdk.NewInt64Coin(paymentDenom, 0)),
			payments: []types.Payment{
				{
					ID:           1,
					Source:       "source1",
					SourceAmount: sdk.NewCoins(sdk.NewInt64Coin(paymentDenom, 100)),
				},
				{
					ID:           2,
					Source:       "source2",
					SourceAmount: sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 100)),
				},
			},
			expectedAccepted: []uint64{1, 2},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			vault := s.setupBaseVault(underlyingDenom, shareDenom, paymentDenom)
			vaultAddr := vault.GetAddress()
			principalAddr := vault.PrincipalMarkerAddress()

			// Setup pending swap-outs
			for i, p := range tc.pendingSwapOuts {
				p.VaultAddress = vaultAddr.String()
				_, err := s.k.PendingSwapOutQueue.Enqueue(s.ctx, s.ctx.BlockTime().Unix(), &p)
				s.Require().NoError(err, "failed to enqueue swap-out %d", i)
			}

			// Setup balances in principal account
			if !tc.balances.IsZero() {
				FundAccount(s.ctx, s.simApp.BankKeeper, principalAddr, tc.balances)
			}

			// Setup Mock Exchange Keeper
			var acceptedIDs []uint64
			s.k.ExchangeKeeper = MockExchangeKeeper{
				IteratePaymentsFn: func(ctx sdk.Context, addr sdk.AccAddress, cb func(payment types.Payment) (stop bool, err error)) error {
					if addr.String() != vaultAddr.String() {
						return nil
					}
					for _, p := range tc.payments {
						stop, err := cb(p)
						if err != nil || stop {
							return err
						}
					}
					return nil
				},
				AcceptPaymentFn: func(ctx sdk.Context, dest sdk.AccAddress, paymentID uint64) error {
					acceptedIDs = append(acceptedIDs, paymentID)
					return nil
				},
			}

			// Call EndBlocker
			err := s.k.EndBlocker(s.ctx)
			s.Require().NoError(err)

			s.Require().ElementsMatch(tc.expectedAccepted, acceptedIDs, "accepted IDs mismatch")
		})
	}
}

// MockExchangeKeeper matches types.ExchangeKeeper
type MockExchangeKeeper struct {
	AcceptPaymentFn  func(ctx sdk.Context, destination sdk.AccAddress, paymentID uint64) error
	IteratePaymentsFn func(ctx sdk.Context, addr sdk.AccAddress, cb func(payment types.Payment) (stop bool, err error)) error
}

func (m MockExchangeKeeper) AcceptPayment(ctx sdk.Context, destination sdk.AccAddress, paymentID uint64) error {
	if m.AcceptPaymentFn != nil {
		return m.AcceptPaymentFn(ctx, destination, paymentID)
	}
	return nil
}

func (m MockExchangeKeeper) IteratePayments(ctx sdk.Context, addr sdk.AccAddress, cb func(payment types.Payment) (stop bool, err error)) error {
	if m.IteratePaymentsFn != nil {
		return m.IteratePaymentsFn(ctx, addr, cb)
	}
	return nil
}
