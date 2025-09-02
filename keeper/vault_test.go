package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
)

type vaultAttrs struct {
	admin      string
	share      string
	underlying string
	payment    string
	expected   types.VaultAccount
}

func (v vaultAttrs) GetAdmin() string           { return v.admin }
func (v vaultAttrs) GetShareDenom() string      { return v.share }
func (v vaultAttrs) GetUnderlyingAsset() string { return v.underlying }
func (v vaultAttrs) GetPaymentDenom() string    { return v.payment }

func (s *TestSuite) TestCreateVault_Success() {
	share := "vaultshare"
	base := "undercoin"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000_000), s.adminAddr)

	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      share,
		underlying: base,
	}

	vault, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err)
	s.Require().Equal(attrs.admin, vault.Admin)
	s.Require().Equal(attrs.share, vault.ShareDenom)
	s.Require().Equal(attrs.underlying, vault.UnderlyingAsset)

	addr := types.GetVaultAddress(share)
	stored, err := s.k.GetVault(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Equal(vault.Address, stored.Address)
}

func (s *TestSuite) TestCreateVault_AssetMarkerMissing() {
	share := "vaultshare"
	base := "missingasset"

	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      share,
		underlying: base,
	}

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().ErrorContains(err, "underlying asset marker")
}

func (s *TestSuite) TestCreateVault_DuplicateMarkerFails() {
	denom := "dupecoin"
	base := "basecoin"

	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1000), s.adminAddr)
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(denom, 1), s.adminAddr)

	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      denom,
		underlying: base,
	}

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().ErrorContains(err, "already exists")
}

func (s *TestSuite) TestCreateVault_InvalidDenomFails() {
	attrs := vaultAttrs{
		admin:      s.adminAddr.String(),
		share:      "!!bad!!",
		underlying: "under",
	}
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(attrs.underlying, 1000), s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().ErrorContains(err, "invalid denom")
}

func (s *TestSuite) TestCreateVault_InvalidAdminFails() {
	share := "vaultx"
	base := "basecoin"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 500), s.adminAddr)

	attrs := vaultAttrs{
		admin:      "not-a-valid-bech32",
		share:      share,
		underlying: base,
	}

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().ErrorContains(err, "invalid admin address")
}

func (s *TestSuite) TestSwapIn_MultiAsset() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	unacceptedDenom := "junk"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)
	vault.SwapInEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	depositorAddr := s.CreateAndFundAccount(sdk.NewInt64Coin(paymentDenom, 1000))
	s.Require().NoError(FundAccount(s.ctx, s.simApp.BankKeeper, depositorAddr, sdk.NewCoins(sdk.NewInt64Coin(unacceptedDenom, 1000))), "should fund depositor with unaccepted denom")

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 1000))), "should fund vault principal with initial TVV")
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(1000))), "should mint initial share supply")

	depositCoin := sdk.NewInt64Coin(paymentDenom, 10)
	mintedShares, err := s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, depositCoin)
	s.Require().NoError(err, "should successfully swap in an accepted payment denom")

	expectedShares := utils.ShareScalar.MulRaw(5)
	s.Require().Equal(expectedShares, mintedShares.Amount, "minted shares should be proportional to the payment denom's value in the underlying asset")
	s.assertBalance(depositorAddr, shareDenom, expectedShares)
	s.assertBalance(vault.PrincipalMarkerAddress(), paymentDenom, math.NewInt(10))

	unacceptedCoin := sdk.NewInt64Coin(unacceptedDenom, 50)
	_, err = s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, unacceptedCoin)
	s.Require().Error(err, "should fail to swap in an unaccepted asset")
	s.Require().ErrorContains(err, "denom not supported for vault", "error should indicate the denom is not accepted")
}

func (s *TestSuite) TestSwapIn_Failures() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)
	depositorAddr := s.CreateAndFundAccount(sdk.NewInt64Coin(underlyingDenom, 100))

	vault.SwapInEnabled = false
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	_, err := s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, sdk.NewInt64Coin(underlyingDenom, 10))
	s.Require().Error(err, "swap in should fail when disabled")
	s.Require().ErrorContains(err, "swaps are not enabled", "error should mention swaps are disabled")

	vault.SwapInEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	_, err = s.k.SwapIn(s.ctx, vault.GetAddress(), depositorAddr, sdk.NewInt64Coin(underlyingDenom, 101))
	s.Require().Error(err, "swap in should fail with insufficient funds")
	s.Require().ErrorContains(err, "insufficient funds", "error should mention insufficient funds")
}

func (s *TestSuite) TestSwapOut_MultiAsset() {
	underlyingDenom := "ylds"
	paymentDenom := "usdc"
	unacceptedDenom := "junk"
	shareDenom := "vshare"
	vault := s.setupSinglePaymentDenomVault(underlyingDenom, shareDenom, paymentDenom, 1, 2)
	vault.SwapOutEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)

	initialShares := utils.ShareScalar.MulRaw(100)
	redeemerAddr := s.CreateAndFundAccount(sdk.NewCoin(shareDenom, initialShares))

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(
		sdk.NewInt64Coin(underlyingDenom, 500),
		sdk.NewInt64Coin(paymentDenom, 500),
	)), "should fund vault principal with liquidity")
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(500))), "should mint initial share supply")

	sharesToRedeemForPayment := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(10))
	_, err := s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeemForPayment, paymentDenom)
	s.Require().NoError(err, "should successfully swap out for an accepted payment denom")
	s.assertBalance(redeemerAddr, paymentDenom, math.NewInt(24))
	s.assertBalance(redeemerAddr, shareDenom, initialShares.Sub(sharesToRedeemForPayment.Amount))

	sharesToRedeemForUnderlying := sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(5))
	_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeemForUnderlying, "")
	s.Require().NoError(err, "should successfully swap out for the default underlying asset when redeem denom is empty")
	s.assertBalance(redeemerAddr, underlyingDenom, math.NewInt(6))

	_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sharesToRedeemForUnderlying, unacceptedDenom)
	s.Require().Error(err, "should fail to swap out for an unaccepted asset")
	s.Require().ErrorContains(err, "denom not supported for vault", "error should indicate the denom is not accepted")
}

func (s *TestSuite) TestSwapOut_Failures() {
	underlyingDenom := "ylds"
	shareDenom := "vshare"
	vault := s.setupBaseVault(underlyingDenom, shareDenom)
	redeemerAddr := s.CreateAndFundAccount(sdk.NewCoin(shareDenom, math.NewInt(100)))

	vault.SwapOutEnabled = false
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	_, err := s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sdk.NewInt64Coin(shareDenom, 10), "")
	s.Require().Error(err, "swap out should fail when disabled")
	s.Require().ErrorContains(err, "swaps are not enabled", "error should mention swaps are disabled")

	vault.SwapOutEnabled = true
	s.k.AuthKeeper.SetAccount(s.ctx, vault)
	_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sdk.NewInt64Coin(shareDenom, 101), "")
	s.Require().Error(err, "swap out should fail with insufficient shares")
	s.Require().ErrorContains(err, "insufficient funds", "error should mention insufficient funds for shares")

	s.Require().NoError(s.k.BankKeeper.SendCoins(s.ctx, s.adminAddr, vault.PrincipalMarkerAddress(), sdk.NewCoins(sdk.NewInt64Coin(underlyingDenom, 10))), "should fund vault principal with a small amount of liquidity")
	s.Require().NoError(s.k.MarkerKeeper.MintCoin(s.ctx, vault.GetAddress(), sdk.NewCoin(shareDenom, utils.ShareScalar.MulRaw(1000))), "should mint a large supply of shares")

	// Corrected test logic: Expect NO error, but a ZERO payout.
	redeemerBalanceBefore := s.simApp.BankKeeper.GetBalance(s.ctx, redeemerAddr, underlyingDenom)
	_, err = s.k.SwapOut(s.ctx, vault.GetAddress(), redeemerAddr, sdk.NewInt64Coin(shareDenom, 50), "")
	s.Require().NoError(err, "swap out should succeed even with low liquidity, yielding a zero payout")
	s.assertBalance(redeemerAddr, underlyingDenom, redeemerBalanceBefore.Amount) //expect no change
}

func (s *TestSuite) TestSetMinMaxInterestRate_NoOp_NoEvent() {
	share := "vaultshare-np"
	base := "undercoin-np"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000), s.adminAddr)

	attrs := vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: base}
	v, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err)

	s.k.UpdateInterestRates(s.ctx, v, "0.10", "0.10")
	s.k.AuthKeeper.SetAccount(s.ctx, v)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMinInterestRate(s.ctx, v, "0.10")
	s.Require().NoError(err)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMinInterestRate(s.ctx, v, "0.10")
	s.Require().NoError(err)
	s.Require().Len(s.ctx.EventManager().Events(), 0)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMaxInterestRate(s.ctx, v, "0.25")
	s.Require().NoError(err)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMaxInterestRate(s.ctx, v, "0.25")
	s.Require().NoError(err)
	s.Require().Len(s.ctx.EventManager().Events(), 0)
}

func (s *TestSuite) TestSetMinInterestRate_ValidationBlocksWhenAboveExistingMax() {
	share := "vaultshare-min-gt-max"
	base := "undercoin-min-gt-max"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000), s.adminAddr)

	attrs := vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: base}
	v, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err)

	s.Require().NoError(s.k.SetMaxInterestRate(s.ctx, v, "0.40"))

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMinInterestRate(s.ctx, v, "0.50")
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "minimum interest rate")
	s.Require().Equal("0.40", v.MaxInterestRate)
	s.Require().Equal("", v.MinInterestRate)
	s.Require().Len(s.ctx.EventManager().Events(), 0)
}

func (s *TestSuite) TestSetMaxInterestRate_ValidationBlocksWhenBelowExistingMin() {
	share := "vaultshare-max-lt-min"
	base := "undercoin-max-lt-min"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000), s.adminAddr)

	attrs := vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: base}
	v, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err)

	s.Require().NoError(s.k.SetMinInterestRate(s.ctx, v, "-0.50"))

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	err = s.k.SetMaxInterestRate(s.ctx, v, "-0.60")
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "minimum interest rate")
	s.Require().Equal("-0.50", v.MinInterestRate)
	s.Require().Equal("", v.MaxInterestRate)
	s.Require().Len(s.ctx.EventManager().Events(), 0)
}

func (s *TestSuite) TestValidateInterestRateLimits() {
	tests := []struct {
		name      string
		min       string
		max       string
		expectErr string
	}{
		{
			name: "both empty => ok",
			min:  "",
			max:  "",
		},
		{
			name: "min empty => ok",
			min:  "",
			max:  "0.25",
		},
		{
			name: "max empty => ok",
			min:  "0.10",
			max:  "",
		},
		{
			name: "equal => ok",
			min:  "0.15",
			max:  "0.15",
		},
		{
			name: "min < max => ok",
			min:  "0.049",
			max:  "0.051",
		},
		{
			name:      "min > max => error",
			min:       "0.60",
			max:       "0.50",
			expectErr: "minimum interest rate",
		},
		{
			name:      "bad min => error",
			min:       "nope",
			max:       "0.10",
			expectErr: "invalid min interest rate",
		},
		{
			name:      "bad max => error",
			min:       "0.10",
			max:       "wat",
			expectErr: "invalid max interest rate",
		},
		{
			name: "zero min, zero max => ok",
			min:  "0",
			max:  "0",
		},
		{
			name: "zero min, positive max => ok",
			min:  "0",
			max:  "0.25",
		},
		{
			name: "high precision => ok",
			min:  "0.123456789012345678",
			max:  "0.223456789012345678",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := s.k.ValidateInterestRateLimits(tc.min, tc.max)
			if tc.expectErr == "" {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectErr)
			}
		})
	}
}

func (s *TestSuite) TestUpdateInterestRate_BoundsEnforced() {
	share := "vaultshare-rate"
	base := "undercoin-rate"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000_000), s.adminAddr)

	attrs := vaultAttrs{admin: s.adminAddr.String(), share: share, underlying: base}
	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err)
	addr := types.GetVaultAddress(share)

	v, err := s.k.GetVault(s.ctx, addr)
	s.Require().NoError(err)

	s.k.UpdateInterestRates(s.ctx, v, "0.10", "0.10")
	s.k.AuthKeeper.SetAccount(s.ctx, v)

	s.Require().NoError(s.k.SetMinInterestRate(s.ctx, v, "0.10"))
	s.Require().NoError(s.k.SetMaxInterestRate(s.ctx, v, "0.50"))

	srv := keeper.NewMsgServer(s.simApp.VaultKeeper)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = srv.UpdateInterestRate(s.ctx, &types.MsgUpdateInterestRateRequest{
		Admin:        s.adminAddr.String(),
		VaultAddress: addr.String(),
		NewRate:      "0.25",
	})
	s.Require().NoError(err)
	v2, err := s.k.GetVault(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Equal("0.25", v2.CurrentInterestRate)
	s.Require().Equal("0.25", v2.DesiredInterestRate)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = srv.UpdateInterestRate(s.ctx, &types.MsgUpdateInterestRateRequest{
		Admin:        s.adminAddr.String(),
		VaultAddress: addr.String(),
		NewRate:      "0.05",
	})
	s.Require().Error(err)

	s.ctx = s.ctx.WithEventManager(sdk.NewEventManager())
	_, err = srv.UpdateInterestRate(s.ctx, &types.MsgUpdateInterestRateRequest{
		Admin:        s.adminAddr.String(),
		VaultAddress: addr.String(),
		NewRate:      "0.60",
	})
	s.Require().Error(err)
}
