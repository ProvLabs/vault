package types

import (
	sdkmath "cosmossdk.io/math"

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

// NewEventSwapIn creates a new NewEventSwapIn.
func NewEventSwapIn(vaultAddress, owner string, assets, shares sdk.Coin) *EventSwapIn {
	return &EventSwapIn{
		VaultAddress:   vaultAddress,
		SharesReceived: shares,
		AmountIn:       assets,
		Owner:          owner,
	}
}

// NewEventSwapOut creates a new NewEventSwapOut.
func NewEventSwapOut(vaultAddress, owner string, assets, shares sdk.Coin) *EventSwapOut {
	return &EventSwapOut{
		VaultAddress: vaultAddress,
		SharesBurned: shares,
		AmountOut:    assets,
		Owner:        owner,
	}
}

// NewEventVaultReconcile creates a new EventVaultReconcile.
func NewEventVaultReconcile(vaultAddress string, principalBefore, principalAfter sdk.Coin, rate string, duration int64, interestEarned sdkmath.Int) *EventVaultReconcile {
	return &EventVaultReconcile{
		VaultAddress:    vaultAddress,
		PrincipalBefore: principalBefore,
		PrincipalAfter:  principalAfter,
		Rate:            rate,
		Time:            duration,
		InterestEarned:  sdk.Coin{Denom: principalBefore.Denom, Amount: interestEarned},
	}
}
