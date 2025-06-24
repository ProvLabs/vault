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
func NewEventVaultCreated(vaultAddress, admin, shareDenom, underlyingAsset string) *EventVaultCreated {
	return &EventVaultCreated{
		VaultAddress:    vaultAddress,
		Admin:           admin,
		ShareDenom:      shareDenom,
		UnderlyingAsset: underlyingAsset,
	}
}
