package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/types"
)

type vaultAttrs struct {
	admin    string
	share    string
	base     string
	expected types.VaultAccount
}

func (v vaultAttrs) GetAdmin() string           { return v.admin }
func (v vaultAttrs) GetShareDenom() string      { return v.share }
func (v vaultAttrs) GetUnderlyingAsset() string { return v.base }

func (s *TestSuite) TestCreateVault_Success() {
	share := "vaultshare"
	base := "undercoin"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 1_000_000), s.adminAddr)

	attrs := vaultAttrs{
		admin: s.adminAddr.String(),
		share: share,
		base:  base,
	}

	vault, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().NoError(err)
	s.Require().Equal(attrs.admin, vault.Admin)
	s.Require().Equal(attrs.share, vault.ShareDenom)
	s.Require().Equal([]string{attrs.base}, vault.UnderlyingAssets)

	addr := types.GetVaultAddress(share)
	stored, err := s.k.GetVault(s.ctx, addr)
	s.Require().NoError(err)
	s.Require().Equal(vault.Address, stored.Address)
}

func (s *TestSuite) TestCreateVault_AssetMarkerMissing() {
	share := "vaultshare"
	base := "missingasset"

	attrs := vaultAttrs{
		admin: s.adminAddr.String(),
		share: share,
		base:  base,
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
		admin: s.adminAddr.String(),
		share: denom,
		base:  base,
	}

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().ErrorContains(err, "already exists")
}

func (s *TestSuite) TestCreateVault_InvalidDenomFails() {
	attrs := vaultAttrs{
		admin: s.adminAddr.String(),
		share: "!!bad!!",
		base:  "under",
	}
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(attrs.base, 1000), s.adminAddr)

	_, err := s.k.CreateVault(s.ctx, attrs)
	s.Require().ErrorContains(err, "invalid denom")
}

func (s *TestSuite) TestCreateVault_InvalidAdminFails() {
	share := "vaultx"
	base := "basecoin"
	s.requireAddFinalizeAndActivateMarker(sdk.NewInt64Coin(base, 500), s.adminAddr)

	attrs := vaultAttrs{
		admin: "not-a-valid-bech32",
		share: share,
		base:  base,
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
