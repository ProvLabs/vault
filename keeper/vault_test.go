package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/keeper"
	"github.com/provlabs/vault/types"
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

// Ensure this compiles under go test without “unused import” issues.
func TestDummy(t *testing.T) {}
