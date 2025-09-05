package types

import (
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewEventVaultCreated creates a new EventVaultCreated event.
func NewEventVaultCreated(vault *VaultAccount) *EventVaultCreated {
	return &EventVaultCreated{
		VaultAddress:    vault.GetAddress().String(),
		Admin:           vault.Admin,
		ShareDenom:      vault.ShareDenom,
		UnderlyingAsset: vault.UnderlyingAsset,
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
// Note: interestEarned does not use NewCoin to avoid panics with negative amounts.
func NewEventVaultReconcile(vaultAddress string, principalBefore, principalAfter sdk.Coin, rate string, time int64, interestEarned sdkmath.Int) *EventVaultReconcile {
	return &EventVaultReconcile{
		VaultAddress:    vaultAddress,
		PrincipalBefore: principalBefore,
		PrincipalAfter:  principalAfter,
		Rate:            rate,
		Time:            time,
		InterestEarned:  sdk.Coin{Denom: principalBefore.Denom, Amount: interestEarned},
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

// NewEventDepositPrincipalFunds creates a new EventPrincipalDeposit event.
func NewEventDepositPrincipalFunds(vaultAddress, admin string, amount sdk.Coin) *EventDepositPrincipalFunds {
	return &EventDepositPrincipalFunds{
		VaultAddress: vaultAddress,
		Admin:        admin,
		Amount:       amount,
	}
}

// NewEventWithdrawPrincipalFunds creates a new EventWithdrawPrincipalFunds event.
func NewEventWithdrawPrincipalFunds(vaultAddress, admin string, amount sdk.Coin) *EventWithdrawPrincipalFunds {
	return &EventWithdrawPrincipalFunds{
		VaultAddress: vaultAddress,
		Admin:        admin,
		Amount:       amount,
	}
}

// NewEventMinInterestRateUpdated creates a new EventMinInterestRateUpdated event.
func NewEventMinInterestRateUpdated(vaultAddress, admin, minRate string) *EventMinInterestRateUpdated {
	return &EventMinInterestRateUpdated{
		VaultAddress: vaultAddress,
		Admin:        admin,
		MinRate:      minRate,
	}
}

// NewEventMaxInterestRateUpdated creates a new EventMaxInterestRateUpdated event.
func NewEventMaxInterestRateUpdated(vaultAddress, admin, maxRate string) *EventMaxInterestRateUpdated {
	return &EventMaxInterestRateUpdated{
		VaultAddress: vaultAddress,
		Admin:        admin,
		MaxRate:      maxRate,
	}
}

// NewEventPendingWithdrawalExpedited creates a new EventPendingWithdrawalExpedited event.
func NewEventPendingWithdrawalExpedited(id uint64, vault, admin string) *EventPendingWithdrawalExpedited {
	return &EventPendingWithdrawalExpedited{
		Id:    id,
		Vault: vault,
		Admin: admin,
	}
}
