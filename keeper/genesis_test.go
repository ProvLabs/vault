package keeper_test

import (
	"fmt"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func (s *TestSuite) TestVaultGenesis_InitAndExport() {
	shareDenom := "vaultshare"
	underlying := "undercoin"
	admin := s.adminAddr.String()
	vaultAddr := types.GetVaultAddress(shareDenom)

	vault := types.VaultAccount{
		BaseAccount:      authtypes.NewBaseAccountWithAddress(vaultAddr),
		Admin:            admin,
		ShareDenom:       shareDenom,
		UnderlyingAssets: []string{underlying},
	}

	genesis := &types.GenesisState{
		Params: types.DefaultParams(),
		Vaults: []types.VaultAccount{vault},
	}

	s.k.InitGenesis(s.ctx, genesis)

	exported := s.k.ExportGenesis(s.ctx)

	s.Require().Equal(genesis.Params, exported.Params)
	s.Require().Len(exported.Vaults, 1)

	exp := exported.Vaults[0]
	s.Require().Equal(vault.GetAddress().String(), exp.GetAddress().String())
	s.Require().Equal(vault.Admin, exp.Admin)
	s.Require().Equal(vault.ShareDenom, exp.ShareDenom)
	s.Require().Equal(vault.UnderlyingAssets, exp.UnderlyingAssets)
}

func (s *TestSuite) TestVaultGenesis_InitGenesis_Failures() {
	s.Run("nil genesis state", func() {
		s.Require().NotPanics(func() {
			s.k.InitGenesis(s.ctx, nil)
		})
	})

	s.Run("invalid vault in account keeper", func() {
		badVault := &types.VaultAccount{
			BaseAccount:      authtypes.NewBaseAccountWithAddress(sdk.AccAddress("badvault___________")),
			Admin:            "invalid_bech32",
			ShareDenom:       "%%bad%%",
			UnderlyingAssets: []string{"under"},
		}
		s.simApp.AccountKeeper.SetAccount(s.ctx, badVault)

		genState := &types.GenesisState{
			Params: types.DefaultParams(),
			Vaults: []types.VaultAccount{},
		}

		s.Require().Panics(func() {
			s.k.InitGenesis(s.ctx, genState)
		})
	})

	s.Run("duplicate vault account with invalid type", func() {
		vault := types.VaultAccount{
			BaseAccount:      authtypes.NewBaseAccountWithAddress(sdk.AccAddress("dupevault__________")),
			Admin:            s.adminAddr.String(),
			ShareDenom:       "vaulttoken",
			UnderlyingAssets: []string{"under"},
		}
		genState := &types.GenesisState{
			Params: types.DefaultParams(),
			Vaults: []types.VaultAccount{vault},
		}

		base := authtypes.NewBaseAccountWithAddress(vault.GetAddress())
		s.simApp.AccountKeeper.SetAccount(s.ctx, base)

		s.Require().PanicsWithError(fmt.Sprintf("failed to set account number for vault %s", vault.Address), func() {
			s.k.InitGenesis(s.ctx, genState)
		})
	})
}

func (s *TestSuite) TestVaultGenesis_ExportGenesis_Failures() {
	s.Require().PanicsWithError("failed to get vault module params: not found", func() {
		s.k.ExportGenesis(s.ctx)
	})
}
