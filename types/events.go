package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewEventDeposit creates a new EventDeposit.
func NewEventDeposit(caller string, owner string, assets sdk.Coins, shares sdk.Coins, vaultID uint32) *EventDeposit {
	return &EventDeposit{
		Caller:  caller,
		Owner:   owner,
		Assets:  assets.String(),
		Shares:  shares.String(),
		VaultId: vaultID,
	}
}

// NewEventWithdraw creates a new EventWithdraw.
func NewEventWithdraw(caller string, receiver string, owner string, assets sdk.Coins, shares sdk.Coins, vaultID uint32) *EventWithdraw {
	return &EventWithdraw{
		Caller:   caller,
		Receiver: receiver,
		Owner:    owner,
		Assets:   assets.String(),
		Shares:   shares.String(),
		VaultId:  vaultID,
	}
}

// NewEventVaultCreated creates a new EventVaultCreated.
func NewEventVaultCreated(vault *VaultAccount) *EventVaultCreated {
	return &EventVaultCreated{
		VaultAddress:     vault.BaseAccount.Address,
		Admin:            vault.Admin,
		ShareDenom:       vault.ShareDenom,
		UnderlyingAssets: vault.UnderlyingAssets,
	}
}

func NewEventSwapIn(vaultAddress, owner string, assets, shares sdk.Coin) *EventSwapIn {
	return &EventSwapIn{
		VaultAddress:   vaultAddress,
		SharesReceived: shares,
		AmountIn:       assets,
		Owner:          owner,
	}
}

func NewEventSwapOut(vaultAddress, owner string, assets, shares sdk.Coin) *EventSwapOut {
	return &EventSwapOut{
		VaultAddress: vaultAddress,
		SharesBurned: shares,
		AmountOut:    assets,
		Owner:        owner,
	}
}
