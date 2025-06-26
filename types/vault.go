package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	proto "github.com/cosmos/gogoproto/proto"
)

var (
	_ sdk.AccountI             = (*Vault)(nil)
	_ authtypes.GenesisAccount = (*Vault)(nil)
)

type VaultAccountI interface {
	proto.Message

	sdk.AccountI
}

// NewVault creates a new vault.
func NewVault(baseAcc *authtypes.BaseAccount, admin string, shareDenom string, underlyingAssets sdk.Coins) *Vault {
	return &Vault{
		BaseAccount:      baseAcc,
		Admin:            admin,
		ShareDenom:       shareDenom,
		UnderlyingAssets: underlyingAssets,
	}
}
