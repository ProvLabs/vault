package keeper

import (
	"fmt"

	"github.com/provlabs/vault/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// MaxSwapOutBatchSize is the maximum number of pending swap-out requests
	// to process in a single EndBlocker invocation. This prevents a large queue
	// from consuming excessive block time and memory. This is a temporary value
	// and we will need to do more analysis on a proper batch size.
	// See https://github.com/ProvLabs/vault/issues/75.
	MaxSwapOutBatchSize = 100
)

// BeginBlocker is a hook that is called at the beginning of every block.
func (k *Keeper) BeginBlocker(ctx sdk.Context) error {
	if err := k.handleVaultInterestTimeouts(ctx); err != nil {
		return fmt.Errorf("handle vault interest timeouts: %w", err)
	}
	if err := k.handleVaultFeeTimeouts(ctx); err != nil {
		return fmt.Errorf("handle vault fee timeouts: %w", err)
	}
	return nil
}

// EndBlocker is a hook that is called at the end of every block.
func (k *Keeper) EndBlocker(ctx sdk.Context) error {
	if err := k.processPendingSwapOuts(ctx, MaxSwapOutBatchSize); err != nil {
		return fmt.Errorf("process pending swap outs: %w", err)
	}

	if err := k.handleReconciledVaults(ctx); err != nil {
		return fmt.Errorf("handle reconciled vaults: %w", err)
	}

	if err := k.handleVaultLiquidity(ctx); err != nil {
		return fmt.Errorf("handle vault liquidity: %w", err)
	}

	return nil
}

// handleVaultLiquidity checks vaults with pending swap-outs and attempts to create
// liquidity by automatically accepting pending buy-payments.
func (k *Keeper) handleVaultLiquidity(ctx sdk.Context) error {
	vaults, err := k.GetVaults(ctx)
	if err != nil {
		return err
	}

	for _, vaultAddr := range vaults {
		vault, err := k.GetVault(ctx, vaultAddr)
		if err != nil || vault == nil {
			continue
		}

		// Identify denoms for which we need liquidity.
		// We always ensure liquidity in the vault's default payment denom (the default for I/O).
		neededDenoms := map[string]bool{
			vault.PaymentDenom: true,
		}

		hasPending := false
		err = k.PendingSwapOutQueue.WalkByVault(ctx, vaultAddr, func(timestamp int64, id uint64, req types.PendingSwapOut) (stop bool, err error) {
			hasPending = true
			if req.RedeemDenom != "" {
				neededDenoms[req.RedeemDenom] = true
			}
			// Stop walk if we've already identified both potential I/O denoms.
			if len(neededDenoms) >= 2 {
				return true, nil
			}
			return false, nil
		})
		if err != nil || !hasPending {
			continue
		}

		// Identify sources providing ANY of the needed denoms where current liquidity is zero.
		sourcesToAccept := make(map[string]bool)
		err = k.ExchangeKeeper.IteratePayments(ctx, vaultAddr, func(payment types.Payment) (stop bool, err error) {
			for _, coin := range payment.SourceAmount {
				if neededDenoms[coin.Denom] {
					liquidity := k.BankKeeper.GetBalance(ctx, vault.PrincipalMarkerAddress(), coin.Denom).Amount
					if liquidity.IsZero() {
						sourcesToAccept[payment.Source] = true
						break
					}
				}
			}
			return false, nil
		})
		if err != nil {
			k.getLogger(ctx).Error("failed to iterate payments for vault liquidity", "vault", vaultAddr, "error", err)
			continue
		}

		for source := range sourcesToAccept {
			_, err := k.AcceptPaymentsFromSource(ctx, vault, source)
			if err != nil {
				k.getLogger(ctx).Error("failed to handle liquidity for vault", "vault", vaultAddr, "source", source, "error", err)
			}
		}
	}

	return nil
}
