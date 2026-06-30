package keeper

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/collections"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/provenance-io/provenance/x/exchange"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/types"
)

// TestAccessor_handleReconciledVaults exposes this keeper's handleReconciledVaults function for unit tests.
func (k Keeper) TestAccessor_handleReconciledVaults(t *testing.T, ctx context.Context) error {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.handleReconciledVaults(sdkCtx)
}

// TestAccessor_handlePayableVaults exposes this keeper's handlePayableVaults function for unit tests.
func (k Keeper) TestAccessor_handlePayableVaults(t *testing.T, ctx context.Context, payouts []*types.VaultAccount) {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.handlePayableVaults(sdkCtx, payouts)
}

// TestAccessor_handleDepletedVaults exposes this keeper's handleDepletedVaults function for unit tests.
func (k Keeper) TestAccessor_handleDepletedVaults(t *testing.T, ctx context.Context, failedPayouts []*types.VaultAccount) {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.handleDepletedVaults(sdkCtx, failedPayouts)
}

// TestAccessor_handleVaultInterestTimeouts exposes this keeper's handleVaultInterestTimeouts function for unit tests.
func (k Keeper) TestAccessor_handleVaultInterestTimeouts(t *testing.T, ctx context.Context) error {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.handleVaultInterestTimeouts(sdkCtx)
}

// TestAccessor_handleVaultFeeTimeouts exposes this keeper's handleVaultFeeTimeouts function for unit tests.
func (k Keeper) TestAccessor_handleVaultFeeTimeouts(t *testing.T, ctx context.Context) error {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.handleVaultFeeTimeouts(sdkCtx)
}

// TestAccessor_processSwapOutJobs exposes this keeper's processSwapOutJobs function for unit tests.
func (k Keeper) TestAccessor_processSwapOutJobs(t *testing.T, ctx context.Context, jobsToProcess []types.PayoutJob) {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.processSwapOutJobs(sdkCtx, jobsToProcess)
}

// TestAccessor_autoPauseVault exposes this keeper's autoPauseVault function for unit tests.
func (k Keeper) TestAccessor_autoPauseVault(t *testing.T, ctx context.Context, vault *types.VaultAccount, reason string) {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.autoPauseVault(sdkCtx, vault, reason)
}

// TestAccessor_reconcileVault exposes this keeper's reconcileVault function for unit tests.
func (k Keeper) TestAccessor_reconcileVault(t *testing.T, ctx context.Context, vault *types.VaultAccount) error {
	t.Helper()
	return k.reconcileVault(sdk.UnwrapSDKContext(ctx), vault)
}

// TestAccessor_processPendingSwapOuts exposes this keeper's processPendingSwapOuts function for unit tests.
func (k Keeper) TestAccessor_processPendingSwapOuts(t *testing.T, ctx context.Context, size int) error {
	t.Helper()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.processPendingSwapOuts(sdkCtx, size)
}

// TestAccessor_processSingleWithdrawal exposes this keeper's processSingleWithdrawal function for unit tests.
func (k Keeper) TestAccessor_processSingleWithdrawal(t *testing.T, ctx context.Context, id uint64, req types.PendingSwapOut, vault types.VaultAccount) error {
	t.Helper()
	return k.processSingleWithdrawal(sdk.UnwrapSDKContext(ctx), id, req, vault)
}

// TestAccessor_refundWithdrawal exposes this keeper's refundWithdrawal function for unit tests.
func (k Keeper) TestAccessor_refundWithdrawal(t *testing.T, ctx context.Context, id uint64, req types.PendingSwapOut, reason string) error {
	t.Helper()
	return k.refundWithdrawal(sdk.UnwrapSDKContext(ctx), id, req, reason)
}

// TestAccessor_setShareDenomNAV exposes this keeper's setShareDenomNAV function for unit tests.
func (k Keeper) TestAccessor_setShareDenomNAV(t *testing.T, ctx context.Context, vault *types.VaultAccount, vaultMarker markertypes.MarkerAccountI, tvv sdkmath.Int) error {
	t.Helper()
	return k.setShareDenomNAV(sdk.UnwrapSDKContext(ctx), vault, vaultMarker, tvv)
}

// NavReferenceVolume exposes the unexported navReferenceVolume bound for unit tests so they can
// assert the scaled-volume behavior against the real constant rather than a duplicated literal.
var NavReferenceVolume = navReferenceVolume

// TestAccessor_checkPayoutRestrictions exposes this keeper's checkPayoutRestrictions function for unit tests.
func (k Keeper) TestAccessor_checkPayoutRestrictions(t *testing.T, ctx context.Context, vault *types.VaultAccount, owner sdk.AccAddress, assets sdk.Coin) error {
	t.Helper()
	return k.checkPayoutRestrictions(sdk.UnwrapSDKContext(ctx), vault, owner, assets)
}

// TestAccessor_checkSettlementNAVGuardrail exposes this keeper's checkSettlementNAVGuardrail function for unit tests.
func (k Keeper) TestAccessor_checkSettlementNAVGuardrail(t *testing.T, ctx context.Context, vault *types.VaultAccount, assetCoin, paymentCoin sdk.Coin) error {
	t.Helper()
	return k.checkSettlementNAVGuardrail(sdk.UnwrapSDKContext(ctx), vault, assetCoin, paymentCoin)
}

// TestAccessor_applySettlementNAV exposes this keeper's applySettlementNAV function for unit tests.
func (k Keeper) TestAccessor_applySettlementNAV(t *testing.T, ctx context.Context, vault *types.VaultAccount, assetCoin, paymentCoin sdk.Coin, direction, signer string) error {
	t.Helper()
	return k.applySettlementNAV(sdk.UnwrapSDKContext(ctx), vault, assetCoin, paymentCoin, direction, signer)
}

// TestAccessor_publishAssetNAVToMarker exposes this keeper's publishAssetNAVToMarker function for unit tests.
func (k Keeper) TestAccessor_publishAssetNAVToMarker(t *testing.T, ctx context.Context, vault *types.VaultAccount, nav types.VaultNAV) error {
	t.Helper()
	return k.publishAssetNAVToMarker(sdk.UnwrapSDKContext(ctx), vault, nav)
}

// TestAccessor_settlementLegCoins exposes the settlementLegCoins function for unit tests.
func (k Keeper) TestAccessor_settlementLegCoins(t *testing.T, payment *exchange.Payment, direction, paymentDenom string) (sdk.Coin, sdk.Coin, error) {
	t.Helper()
	return settlementLegCoins(payment, direction, paymentDenom)
}

// TestAccessor_stageFromPrincipal exposes this keeper's stageFromPrincipal function for unit tests.
func (k Keeper) TestAccessor_stageFromPrincipal(t *testing.T, ctx context.Context, vault *types.VaultAccount, amt sdk.Coins) error {
	t.Helper()
	return k.stageFromPrincipal(sdk.UnwrapSDKContext(ctx), vault, amt)
}

// TestAccessor_returnToPrincipal exposes this keeper's returnToPrincipal function for unit tests.
func (k Keeper) TestAccessor_returnToPrincipal(t *testing.T, ctx context.Context, vault *types.VaultAccount, amt sdk.Coins) error {
	t.Helper()
	return k.returnToPrincipal(sdk.UnwrapSDKContext(ctx), vault, amt)
}

// TestAccessor_corruptVaultNAV writes undecodable bytes at the internal NAV entry for
// vaultAddr/denom so unit tests can exercise NAV lookup failures other than not-found.
func (k Keeper) TestAccessor_corruptVaultNAV(t *testing.T, ctx context.Context, vaultAddr sdk.AccAddress, denom string) error {
	t.Helper()
	keyCodec := collections.PairKeyCodec(sdk.AccAddressKey, collections.StringKey)
	key := collections.Join(vaultAddr, denom)
	buf := make([]byte, keyCodec.Size(key))
	if _, err := keyCodec.Encode(buf, key); err != nil {
		return fmt.Errorf("failed to encode NAV key for denom %q: %w", denom, err)
	}
	if err := k.storeService.OpenKVStore(ctx).Set(append(types.NAVsKeyPrefix.Bytes(), buf...), []byte{0xFF}); err != nil {
		return fmt.Errorf("failed to write corrupt NAV bytes for denom %q: %w", denom, err)
	}
	return nil
}
