package keeper_test

import (
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
	underlying := "under"
	share := "vaultshares"
	navDenom := "rwa"
	vaultAddr := types.GetVaultAddress(share)
	vault := s.setupBaseVault(underlying, share)

	firstHeight := int64(10)
	firstTime := s.ctx.BlockTime()
	s.ctx = s.ctx.WithBlockHeight(firstHeight)

	firstNav := types.VaultNAV{
		Denom:  navDenom,
		Price:  sdk.NewInt64Coin(underlying, 100),
		Volume: sdkmath.NewInt(5),
		Source: "oracle-first",
	}
	s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, firstNav, s.adminAddr.String()), "first SetVaultNAV should succeed")

	storedFirst, err := s.k.GetVaultNAV(s.ctx, vaultAddr, navDenom)
	s.Require().NoError(err, "GetVaultNAV should return the first entry")
	s.Assert().Equal(firstHeight, storedFirst.UpdatedBlockHeight, "first write should stamp the block height")
	s.Assert().Equal(firstTime.UTC(), storedFirst.UpdatedTime, "first write should stamp the block time")

	secondHeight := int64(20)
	secondTime := firstTime.Add(0).UTC()
	s.ctx = s.ctx.WithBlockHeight(secondHeight)
	// Advance block time by a meaningful offset to distinguish from first write.
	s.ctx = s.ctx.WithBlockTime(firstTime.Add(0))
	secondTime = s.ctx.BlockTime().UTC()

	secondNav := types.VaultNAV{
		Denom:  navDenom,
		Price:  sdk.NewInt64Coin(underlying, 200),
		Volume: sdkmath.NewInt(10),
		Source: "oracle-second",
	}
	s.Require().NoError(s.k.SetVaultNAV(s.ctx, vault, secondNav, s.adminAddr.String()), "second SetVaultNAV should succeed")

	storedSecond, err := s.k.GetVaultNAV(s.ctx, vaultAddr, navDenom)
	s.Require().NoError(err, "GetVaultNAV should return the overwritten entry")
	s.Assert().Equal(secondHeight, storedSecond.UpdatedBlockHeight, "overwrite should re-stamp the block height to the second write height")
	s.Assert().Equal(secondTime, storedSecond.UpdatedTime, "overwrite should re-stamp the block time to the second write time")
	s.Assert().Equal(sdk.NewInt64Coin(underlying, 200), storedSecond.Price, "overwrite should update the price")
	s.Assert().Equal(sdkmath.NewInt(10), storedSecond.Volume, "overwrite should update the volume")
	s.Assert().Equal("oracle-second", storedSecond.Source, "overwrite should update the source")
}
