package keeper_test

import (
	"fmt"

	"github.com/provlabs/vault/types"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/keeper"
)

func (s *TestSuite) TestAcceptPayments_Table() {
	underlyingDenom := "nhash"
	shareDenom := "svnhash"
	issuerAddr := sdk.AccAddress("issuer______________")
	otherAddr := sdk.AccAddress("other_______________")
	scopeDenom := "scope1qzscope" // Removed slashes to avoid SDK validation issues if any remain

	type testCase struct {
		name               string
		setup              func(vaultAddr sdk.AccAddress)
		authority          sdk.AccAddress
		source             sdk.AccAddress
		payments           []types.Payment
		expectedAccepted   []uint64
		expectedNAV        map[string]sdkmath.Int
		expectedError      string
	}

	cases := []testCase{
		{
			name: "success - accept buy payment from issuer",
			authority: s.adminAddr,
			source: issuerAddr,
			payments: []types.Payment{
				{
					ID:           1,
					Source:       issuerAddr.String(),
					SourceAmount: sdk.Coins{sdk.Coin{Denom: scopeDenom, Amount: sdkmath.NewInt(1)}},
					TargetAmount: sdk.NewCoins(sdk.NewCoin(underlyingDenom, sdkmath.NewInt(1000))),
				},
			},
			expectedAccepted: []uint64{1},
			expectedNAV: map[string]sdkmath.Int{
				scopeDenom: sdkmath.NewInt(1000),
			},
		},
		{
			name: "success - multiple payments, filter by source",
			authority: s.adminAddr,
			source: issuerAddr,
			payments: []types.Payment{
				{
					ID:           1,
					Source:       issuerAddr.String(),
					SourceAmount: sdk.Coins{sdk.Coin{Denom: scopeDenom, Amount: sdkmath.NewInt(1)}},
					TargetAmount: sdk.NewCoins(sdk.NewCoin(underlyingDenom, sdkmath.NewInt(1000))),
				},
				{
					ID:           2,
					Source:       otherAddr.String(),
					SourceAmount: sdk.Coins{sdk.Coin{Denom: scopeDenom, Amount: sdkmath.NewInt(1)}},
					TargetAmount: sdk.NewCoins(sdk.NewCoin(underlyingDenom, sdkmath.NewInt(1100))),
				},
			},
			expectedAccepted: []uint64{1},
			expectedNAV: map[string]sdkmath.Int{
				scopeDenom: sdkmath.NewInt(1000),
			},
		},
		{
			name: "error - unauthorized authority",
			authority: otherAddr,
			source: issuerAddr,
			expectedError: "unauthorized",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest() // Reset for each case
			
			vault := s.createStandardVault(underlyingDenom, shareDenom, s.adminAddr)
			vaultAddr := vault.GetAddress()
			principalAddr := vault.PrincipalMarkerAddress()

			// Setup Mock Exchange Keeper
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
					// Verify bypass
					if ctx.Context().Value("quarantine-bypass") != true {
						return fmt.Errorf("missing quarantine bypass")
					}
					// Find the payment
					var payment *types.Payment
					for _, p := range tc.payments {
						if p.ID == paymentID {
							payment = &p
							break
						}
					}
					if payment == nil {
						return fmt.Errorf("payment not found")
					}
					sourceAddr := sdk.MustAccAddressFromBech32(payment.Source)
					// Simulate exchange: Move SourceAmount from Source to Destination (vault)
					if err := s.simApp.BankKeeper.SendCoins(ctx, sourceAddr, dest, payment.SourceAmount); err != nil {
						return err
					}
					// Simulate exchange: Move TargetAmount from Destination (vault) to Source
					if err := s.simApp.BankKeeper.SendCoins(ctx, dest, sourceAddr, payment.TargetAmount); err != nil {
						return err
					}
					return nil
				},
			}

			// Fund source for SourceAmount
			for _, p := range tc.payments {
				if p.Source == tc.source.String() {
					s.FundAccount(sdk.MustAccAddressFromBech32(p.Source), p.SourceAmount...)
				}
			}

			// Fund vault principal for TargetAmount
			for _, p := range tc.payments {
				if p.Source == tc.source.String() && !p.TargetAmount.IsZero() {
					s.FundAccount(principalAddr, p.TargetAmount...)
				}
			}

			req := &types.MsgAcceptPaymentsRequest{
				Authority:    tc.authority.String(),
				VaultAddress: vaultAddr.String(),
				Source:       tc.source.String(),
			}

			res, err := keeper.NewMsgServer(s.k).AcceptPayments(s.ctx, req)

			if tc.expectedError != "" {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectedError)
				return
			}

			s.Require().NoError(err)
			s.Require().ElementsMatch(tc.expectedAccepted, res.AcceptedPaymentIds)

			// Verify NAV updates
			for denom, expectedPrice := range tc.expectedNAV {
				nav, err := s.k.AssetNAV.Get(s.ctx, collections.Join(vaultAddr, denom))
				s.Require().NoError(err)
				s.Require().Equal(expectedPrice.String(), nav.Price.Amount.String())
			}

			// Verify funds moved to principal
			for _, p := range tc.payments {
				if p.Source == tc.source.String() {
					for _, coin := range p.SourceAmount {
						s.assertBalance(principalAddr, coin.Denom, coin.Amount)
					}
					// Verify vault account is empty
					s.assertBalance(vaultAddr, underlyingDenom, sdkmath.ZeroInt())
				}
			}
		})
	}
}

func (s *TestSuite) TestUpdateAssetNAV() {
	underlyingDenom := "nhash"
	shareDenom := "svnhash"
	scopeDenom := "scope1qzscope"
	
	s.SetupTest()
	vault := s.createStandardVault(underlyingDenom, shareDenom, s.adminAddr)
	vaultAddr := vault.GetAddress()

	msgServer := keeper.NewMsgServer(s.k)

	// Update NAV
	price := sdk.NewCoin(underlyingDenom, sdkmath.NewInt(1500))
	req := &types.MsgUpdateAssetNAVRequest{
		Authority:    s.adminAddr.String(),
		VaultAddress: vaultAddr.String(),
		AssetDenom:   scopeDenom,
		AssetPrice:   price,
	}

	_, err := msgServer.UpdateAssetNAV(s.ctx, req)
	s.Require().NoError(err)

	// Verify NAV in state
	nav, err := s.k.AssetNAV.Get(s.ctx, collections.Join(vaultAddr, scopeDenom))
	s.Require().NoError(err)
	s.Require().Equal(price.Amount.String(), nav.Price.Amount.String())
	s.Require().Equal(scopeDenom, nav.AssetDenom)

	// Verify it affects UnitPriceFraction
	num, den, err := s.k.UnitPriceFraction(s.ctx, scopeDenom, *vault)
	s.Require().NoError(err)
	s.Require().Equal(price.Amount.String(), num.String())
	s.Require().Equal("1", den.String())
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

// Helper to fund account in tests
func (s *TestSuite) FundAccount(addr sdk.AccAddress, coins ...sdk.Coin) {
	FundAccount(s.ctx, s.simApp.BankKeeper, addr, sdk.NewCoins(coins...))
}

// Helper to create a vault
func (s *TestSuite) createStandardVault(underlying, share string, admin sdk.AccAddress) *types.VaultAccount {
	// Setup markers
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(underlying, 0), admin)
	// Create vault
	req := &types.MsgCreateVaultRequest{
		Admin:           admin.String(),
		UnderlyingAsset: underlying,
		ShareDenom:      share,
	}
	res, err := keeper.NewMsgServer(s.k).CreateVault(s.ctx, req)
	s.Require().NoError(err)
	
	vaultAddr := sdk.MustAccAddressFromBech32(res.VaultAddress)
	vault, err := s.k.GetVault(s.ctx, vaultAddr)
	s.Require().NoError(err)
	return vault
}
