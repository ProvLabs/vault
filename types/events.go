package types

import (
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewEventVaultCreated creates a new EventVaultCreated event.
func NewEventVaultCreated(vault *VaultAccount) *EventVaultCreated {
	return &EventVaultCreated{
		VaultAddress:     vault.GetAddress().String(),
		Admin:            vault.Admin,
		ShareDenom:       vault.ShareDenom,
		UnderlyingAssets: vault.UnderlyingAssets,
	}
}

// NewEventSwapIn creates a new EventSwapIn event.
func NewEventSwapIn(vaultAddress, owner string, amountIn, sharesReceived sdk.Coin) *EventSwapIn {
	return &EventSwapIn{
		VaultAddress:   vaultAddress,
		Owner:          owner,
		AmountIn:       amountIn,
		SharesReceived: sharesReceived,
	}
}

// NewEventSwapOut creates a new EventSwapOut event.
func NewEventSwapOut(vaultAddress, owner string, amountOut, sharesBurned sdk.Coin) *EventSwapOut {
	return &EventSwapOut{
		VaultAddress: vaultAddress,
		Owner:        owner,
		AmountOut:    amountOut,
		SharesBurned: sharesBurned,
	}
}

// NewEventVaultReconcile creates a new EventVaultReconcile event.
func NewEventVaultReconcile(vaultAddress string, principalBefore, principalAfter sdk.Coin, rate string, time int64, interestEarned sdkmath.Int) *EventVaultReconcile {
	return &EventVaultReconcile{
		VaultAddress:    vaultAddress,
		PrincipalBefore: principalBefore,
		PrincipalAfter:  principalAfter,
		Rate:            rate,
		Time:            time,
		InterestEarned:  sdk.NewCoin(principalBefore.Denom, interestEarned),
	}
}

// NewEventVaultInterestChange creates a new EventVaultInterestChange event.
func NewEventVaultInterestChange(vaultAddress, currentRate, desiredRate string) *EventVaultInterestChange {
	return &EventVaultInterestChange{
		VaultAddress: vaultAddress,
		CurrentRate:  currentRate,
		DesiredRate:  desiredRate,
	}
}

// NewEventInterestDeposit creates a new EventInterestDeposit event.
func NewEventInterestDeposit(vaultAddress, admin string, amount sdk.Coin) *EventInterestDeposit {
	return &EventInterestDeposit{
		VaultAddress: vaultAddress,
		Admin:        admin,
		Amount:       amount,
	}
}

// NewEventInterestWithdrawal creates a new EventInterestWithdrawal event.
func NewEventInterestWithdrawal(vaultAddress, admin string, amount sdk.Coin) *EventInterestWithdrawal {
	return &EventInterestWithdrawal{
		VaultAddress: vaultAddress,
		Admin:        admin,
		Amount:       amount,
	}
}

// NewEventToggleSwapIn creates a new EventToggleSwapIn event.
func NewEventToggleSwapIn(vaultAddress, admin string, enabled bool) *EventToggleSwapIn {
	return &EventToggleSwapIn{
		VaultAddress: vaultAddress,
		Admin:        admin,
		Enabled:      enabled,
	}
}

// NewEventToggleSwapOut creates a new EventToggleSwapOut event.
func NewEventToggleSwapOut(vaultAddress, admin string, enabled bool) *EventToggleSwapOut {
	return &EventToggleSwapOut{
		VaultAddress: vaultAddress,
		Admin:        admin,
		Enabled:      enabled,
	}
}
