package types

import (
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	RefundReasonInsufficientFunds          = "insufficient_funds"
	RefundReasonPermissionDenied           = "permission_denied"
	RefundReasonMarkerNotActive            = "marker_not_active"
	RefundReasonRecipientMissingAttributes = "recipient_missing_required_attributes"
	RefundReasonRecipientInvalid           = "recipient_invalid"
	RefundReasonNavNotFound                = "nav_not_found"
	RefundReasonReconcileFailure           = "reconcile_failure"
	RefundReasonUnknown                    = "unknown_error"
)

// NewEventVaultCreated creates a new EventVaultCreated event.
func NewEventVaultCreated(vault *VaultAccount) *EventVaultCreated {
	return &EventVaultCreated{
		VaultAddress:    vault.GetAddress().String(),
		Admin:           vault.Admin,
		ShareDenom:      vault.TotalShares.Denom,
		UnderlyingAsset: vault.UnderlyingAsset,
	}
}

// NewEventSwapIn creates a new EventSwapIn event.
func NewEventSwapIn(vaultAddress, owner string, amountIn, sharesReceived sdk.Coin) *EventSwapIn {
	return &EventSwapIn{
		VaultAddress:   vaultAddress,
		Owner:          owner,
		AmountIn:       amountIn.String(),
		SharesReceived: sharesReceived.String(),
	}
}

// NewEventSwapOut creates a new EventSwapOut event.
func NewEventSwapOut(vaultAddress, owner string, amountOut, sharesBurned sdk.Coin) *EventSwapOut {
	return &EventSwapOut{
		VaultAddress: vaultAddress,
		Owner:        owner,
		AmountOut:    amountOut.String(),
		SharesBurned: sharesBurned.String(),
	}
}

// NewEventVaultReconcile creates a new EventVaultReconcile event.
// Note: interestEarned does not use NewCoin to avoid panics with negative amounts.
func NewEventVaultReconcile(vaultAddress string, principalBefore, principalAfter sdk.Coin, rate string, time int64, interestEarned sdkmath.Int) *EventVaultReconcile {
	interestEarnedCoin := sdk.Coin{
		Denom:  principalBefore.Denom,
		Amount: interestEarned,
	}
	return &EventVaultReconcile{
		VaultAddress:    vaultAddress,
		PrincipalBefore: principalBefore.String(),
		PrincipalAfter:  principalAfter.String(),
		Rate:            rate,
		Time:            time,
		InterestEarned:  interestEarnedCoin.String(),
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
func NewEventInterestDeposit(vaultAddress, authority string, amount sdk.Coin) *EventInterestDeposit {
	return &EventInterestDeposit{
		VaultAddress: vaultAddress,
		Authority:    authority,
		Amount:       amount.String(),
	}
}

// NewEventInterestWithdrawal creates a new EventInterestWithdrawal event.
func NewEventInterestWithdrawal(vaultAddress, authority string, amount sdk.Coin) *EventInterestWithdrawal {
	return &EventInterestWithdrawal{
		VaultAddress: vaultAddress,
		Authority:    authority,
		Amount:       amount.String(),
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
func NewEventDepositPrincipalFunds(vaultAddress, authority string, amount sdk.Coin) *EventDepositPrincipalFunds {
	return &EventDepositPrincipalFunds{
		VaultAddress: vaultAddress,
		Authority:    authority,
		Amount:       amount.String(),
	}
}

// NewEventWithdrawPrincipalFunds creates a new EventWithdrawPrincipalFunds event.
func NewEventWithdrawPrincipalFunds(vaultAddress, authority string, amount sdk.Coin) *EventWithdrawPrincipalFunds {
	return &EventWithdrawPrincipalFunds{
		VaultAddress: vaultAddress,
		Authority:    authority,
		Amount:       amount.String(),
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

// NewEventSwapOutRequested creates a new EventSwapOutRequested event.
func NewEventSwapOutRequested(vaultAddress, owner, redeemDenom string, shares sdk.Coin, requestID uint64) *EventSwapOutRequested {
	return &EventSwapOutRequested{
		VaultAddress: vaultAddress,
		Owner:        owner,
		RedeemDenom:  redeemDenom,
		Shares:       shares.String(),
		RequestId:    requestID,
	}
}

// NewEventSwapOutCompleted creates a new EventSwapOutCompleted event.
func NewEventSwapOutCompleted(vaultAddress, owner string, assets sdk.Coin, requestID uint64) *EventSwapOutCompleted {
	return &EventSwapOutCompleted{
		VaultAddress: vaultAddress,
		Owner:        owner,
		Assets:       assets.String(),
		RequestId:    requestID,
	}
}

// NewEventSwapOutRefunded creates a new EventSwapOutRefunded event.
func NewEventSwapOutRefunded(vaultAddress, owner string, shares sdk.Coin, requestID uint64, reason string) *EventSwapOutRefunded {
	return &EventSwapOutRefunded{
		VaultAddress: vaultAddress,
		Owner:        owner,
		Shares:       shares.String(),
		RequestId:    requestID,
		Reason:       reason,
	}
}

// NewEventPendingSwapOutExpedited creates a new EventPendingSwapOutExpedited event.
func NewEventPendingSwapOutExpedited(requestID uint64, vault, authority string) *EventPendingSwapOutExpedited {
	return &EventPendingSwapOutExpedited{
		RequestId: requestID,
		Vault:     vault,
		Authority: authority,
	}
}

// NewEventVaultPaused creates a new EventVaultPaused event.
func NewEventVaultPaused(vaultAddress, authority, reason string, totalVaultValue sdk.Coin) *EventVaultPaused {
	return &EventVaultPaused{
		VaultAddress:    vaultAddress,
		Authority:       authority,
		Reason:          reason,
		TotalVaultValue: totalVaultValue.String(),
	}
}

// NewEventVaultUnpaused creates a new EventVaultUnpaused event.
func NewEventVaultUnpaused(vaultAddress, authority string, totalVaultValue sdk.Coin) *EventVaultUnpaused {
	return &EventVaultUnpaused{
		VaultAddress:    vaultAddress,
		Authority:       authority,
		TotalVaultValue: totalVaultValue.String(),
	}
}

// NewEventBridgeAddressSet creates a new EventBridgeAddressSet event.
func NewEventBridgeAddressSet(vaultAddress, admin, bridgeAddress string) *EventBridgeAddressSet {
	return &EventBridgeAddressSet{
		VaultAddress:  vaultAddress,
		Admin:         admin,
		BridgeAddress: bridgeAddress,
	}
}

// NewEventBridgeToggled creates a new EventBridgeToggled event.
func NewEventBridgeToggled(vaultAddress, admin string, enabled bool) *EventBridgeToggled {
	return &EventBridgeToggled{
		VaultAddress: vaultAddress,
		Admin:        admin,
		Enabled:      enabled,
	}
}

// NewEventBridgeMintShares creates a new EventBridgeMintShares event.
func NewEventBridgeMintShares(vaultAddress, bridge string, shares sdk.Coin) *EventBridgeMintShares {
	return &EventBridgeMintShares{
		VaultAddress: vaultAddress,
		Bridge:       bridge,
		Shares:       shares.String(),
	}
}

// NewEventBridgeBurnShares creates a new EventBridgeBurnShares event.
func NewEventBridgeBurnShares(vaultAddress, bridge string, shares sdk.Coin) *EventBridgeBurnShares {
	return &EventBridgeBurnShares{
		VaultAddress: vaultAddress,
		Bridge:       bridge,
		Shares:       shares.String(),
	}
}

// NewEventAssetManagerSet creates a new EventAssetManagerSet event.
func NewEventAssetManagerSet(vaultAddress, admin, assetManager string) *EventAssetManagerSet {
	return &EventAssetManagerSet{
		VaultAddress: vaultAddress,
		Admin:        admin,
		AssetManager: assetManager,
	}
}
