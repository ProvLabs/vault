package keeper_test

import (
	"time"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/provlabs/vault/types"
)

// TestKeeper_GetVaultNAV_NotFound verifies GetVaultNAV returns collections.ErrNotFound
// when no entry exists for the given vault address and denom.
func (s *TestSuite) TestKeeper_GetVaultNAV_NotFound() {
	underlying := "under"
	share := "vaultshares"
	vaultAddr := types.GetVaultAddress(share)
	s.setupBaseVault(underlying, share)

	tests := []struct {
		name      string
		vaultAddr sdk.AccAddress
		denom     string
	}{
		{
			name:      "denom has no entry for the vault",
			vaultAddr: vaultAddr,
			denom:     "rwa",
		},
		{
			name:      "completely unknown vault address has no entry",
			vaultAddr: types.GetVaultAddress("unknownshare"),
			denom:     "rwa",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			_, err := s.k.GetVaultNAV(s.ctx, tc.vaultAddr, tc.denom)
			s.Require().Error(err, "GetVaultNAV should return an error when no entry exists for vault %s denom %s", tc.vaultAddr, tc.denom)
			s.Assert().ErrorIs(err, collections.ErrNotFound, "GetVaultNAV should return collections.ErrNotFound for vault %s denom %s", tc.vaultAddr, tc.denom)
		})
	}
}

// TestKeeper_SetVaultNAV_OverwriteReStamps verifies that a second SetVaultNAV call
// updates the block height and time on the stored entry rather than preserving the
// values from the first write.
func (s *TestSuite) TestKeeper_SetVaultNAV_OverwriteReStamps() {
	type wantFields struct {
		price  sdk.Coin
		volume sdkmath.Int
		source string
	}
	cases := []struct {
		name         string
		underlying   string
		share        string
		navDenom     string
		firstHeight  int64
		firstNav     types.VaultNAV
		secondHeight int64
		secondNav    types.VaultNAV
		want         wantFields
	}{
		{
			name:        "overwrite re-stamps height, time, price, volume, and source",
			underlying:  "under",
			share:       "vaultshares",
			navDenom:    "rwa",
			firstHeight: 10,
			firstNav: types.VaultNAV{
				Denom:  "rwa",
				Price:  sdk.NewInt64Coin("under", 100),
				Volume: sdkmath.NewInt(5),
				Source: "oracle-first",
			},
			secondHeight: 20,
			secondNav: types.VaultNAV{
				Denom:  "rwa",
				Price:  sdk.NewInt64Coin("under", 200),
				Volume: sdkmath.NewInt(10),
				Source: "oracle-second",
			},
			want: wantFields{
				price:  sdk.NewInt64Coin("under", 200),
				volume: sdkmath.NewInt(10),
				source: "oracle-second",
			},
		},
		{
			name:        "overwrite with same source still re-stamps block metadata",
			underlying:  "usdc",
			share:       "usdcshares",
			navDenom:    "bond",
			firstHeight: 1,
			firstNav: types.VaultNAV{
				Denom:  "bond",
				Price:  sdk.NewInt64Coin("usdc", 1_000),
				Volume: sdkmath.NewInt(1),
				Source: "static-oracle",
			},
			secondHeight: 2,
			secondNav: types.VaultNAV{
				Denom:  "bond",
				Price:  sdk.NewInt64Coin("usdc", 1_050),
				Volume: sdkmath.NewInt(2),
				Source: "static-oracle",
			},
			want: wantFields{
				price:  sdk.NewInt64Coin("usdc", 1_050),
				volume: sdkmath.NewInt(2),
				source: "static-oracle",
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			vault := s.setupBaseVault(tc.underlying, tc.share)
			vaultAddr := types.GetVaultAddress(tc.share)
			s.requireSimpleMarker(tc.navDenom)

			baseCtx := s.ctx
			firstTime := baseCtx.BlockTime().UTC()
			ctx1 := baseCtx.WithBlockHeight(tc.firstHeight)

			s.Require().NoError(s.k.SetVaultNAV(ctx1, vault, tc.firstNav, s.adminAddr.String()), "first SetVaultNAV should succeed")

			storedFirst, err := s.k.GetVaultNAV(ctx1, vaultAddr, tc.navDenom)
			s.Require().NoError(err, "GetVaultNAV should return the first entry")
			s.Assert().Equal(tc.firstHeight, storedFirst.UpdatedBlockHeight, "first write should stamp the block height")
			s.Assert().Equal(firstTime, storedFirst.UpdatedTime, "first write should stamp the block time")

			secondTime := firstTime.Add(time.Second)
			ctx2 := ctx1.WithBlockHeight(tc.secondHeight).WithBlockTime(secondTime)

			s.Require().NoError(s.k.SetVaultNAV(ctx2, vault, tc.secondNav, s.adminAddr.String()), "second SetVaultNAV should succeed")

			storedSecond, err := s.k.GetVaultNAV(ctx2, vaultAddr, tc.navDenom)
			s.Require().NoError(err, "GetVaultNAV should return the overwritten entry")
			s.Assert().Equal(tc.secondHeight, storedSecond.UpdatedBlockHeight, "overwrite should re-stamp the block height")
			s.Assert().Equal(secondTime.UTC(), storedSecond.UpdatedTime, "overwrite should re-stamp the block time")
			s.Assert().Equal(tc.want.price, storedSecond.Price, "overwrite should update the price")
			s.Assert().Equal(tc.want.volume, storedSecond.Volume, "overwrite should update the volume")
			s.Assert().Equal(tc.want.source, storedSecond.Source, "overwrite should update the source")
		})
	}
}

// TestKeeper_SetVaultNAV_RejectsInvalidInput verifies SetVaultNAV rejects every
// invalid input before persisting an entry: the vault share denom, an invalid
// price coin, a non-positive price amount, a price denom outside the vault's
// accepted denoms, a nil or non-positive volume, and a denom that is not a
// registered marker.
func (s *TestSuite) TestKeeper_SetVaultNAV_RejectsInvalidInput() {
	underlying := "under"
	share := "vaultshares"
	registeredDenom := "rwa"
	vault := s.setupBaseVault(underlying, share)
	vaultAddr := types.GetVaultAddress(share)
	s.requireSimpleMarker(registeredDenom)

	tests := []struct {
		name              string
		nav               types.VaultNAV
		expectedErrSubstr string
	}{
		{
			name: "rejects the vault share denom",
			nav: types.VaultNAV{
				Denom:  share,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "cannot set NAV for vault share denom",
		},
		{
			name: "rejects an invalid price coin",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.Coin{Denom: underlying, Amount: sdkmath.NewInt(-1)},
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "invalid NAV price",
		},
		{
			name: "rejects a zero price amount",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 0),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "NAV price amount must be positive",
		},
		{
			name: "rejects a price denom outside the vault accepted denoms",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin("notaccepted", 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "must be an accepted vault denom",
		},
		{
			name: "rejects a nil volume",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.Int{},
			},
			expectedErrSubstr: "NAV volume must be positive",
		},
		{
			name: "rejects a zero volume",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.ZeroInt(),
			},
			expectedErrSubstr: "NAV volume must be positive",
		},
		{
			name: "rejects a negative volume",
			nav: types.VaultNAV{
				Denom:  registeredDenom,
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(-1),
			},
			expectedErrSubstr: "NAV volume must be positive",
		},
		{
			name: "rejects a denom that is not a registered marker",
			nav: types.VaultNAV{
				Denom:  "ghostdenom",
				Price:  sdk.NewInt64Coin(underlying, 100),
				Volume: sdkmath.NewInt(1),
			},
			expectedErrSubstr: "is not a registered marker",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := s.k.SetVaultNAV(s.ctx, vault, tc.nav, s.adminAddr.String())
			s.Require().Error(err, "SetVaultNAV should reject input for case %q", tc.name)
			s.Assert().Contains(err.Error(), tc.expectedErrSubstr, "SetVaultNAV error for case %q should mention %q", tc.name, tc.expectedErrSubstr)

			_, getErr := s.k.GetVaultNAV(s.ctx, vaultAddr, tc.nav.Denom)
			s.Assert().ErrorIs(getErr, collections.ErrNotFound, "SetVaultNAV must not persist an entry for rejected input %q", tc.name)
		})
	}
}
