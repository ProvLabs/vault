package types

import (
	fmt "fmt"

	"github.com/cometbft/cometbft/crypto"

	"cosmossdk.io/collections"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "vault"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	// It should be synced with the gov module's name if it is ever changed.
	// See: https://github.com/cosmos/cosmos-sdk/blob/v0.52.0-beta.2/x/gov/types/keys.go#L9
	GovModuleName = "gov"

	// AUMFeeRate is the 15 bps technology fee rate (0.15%).
	AUMFeeRate = "0.0015"

	// ProvLabsMainnetFeeAddress is the hardcoded ProvLabs fee collection address for pio-mainnet-1.
	ProvLabsMainnetFeeAddress = "pb1evyv7neax9qtxxzuexnhylxyz4guvsyjjqke4h"
	// ProvLabsTestnetFeeAddress is the hardcoded ProvLabs fee collection address for pio-testnet-1.
	ProvLabsTestnetFeeAddress = "tp19ftpcggezgal5ascglq5m022z4e453kh6c9g5f"
)

var (
	// VaultsKeyPrefix is the prefix to retrieve all Vaults
	VaultsKeyPrefix = collections.NewPrefix(0)
	// VaultsName is a human-readable name for the vaults collection.
	VaultsName = "vaults"
	// VaultPayoutVerificationSetPrefix is the prefix to retrieve all VaultPayoutVerificationQueue.
	VaultPayoutVerificationSetPrefix = collections.NewPrefix(1)
	// VaultPayoutVerificationSetName is a human-readable name for the vault payout verification set collection.
	VaultPayoutVerificationSetName = "vault_payout_verification_set"
	// VaultPayoutTimeoutQueuePrefix is the prefix to retrieve all VaultPayoutTimeoutQueue.
	VaultPayoutTimeoutQueuePrefix = collections.NewPrefix(2)
	// VaultPayoutTimeoutQueueName is a human-readable name for the payout timeout queue collection.
	VaultPayoutTimeoutQueueName = "vault_payout_timeout_queue"
	// VaultFeeTimeoutQueuePrefix is the prefix to retrieve all VaultFeeTimeoutQueue.
	VaultFeeTimeoutQueuePrefix = collections.NewPrefix(7)
	// VaultFeeTimeoutQueueName is a human-readable name for the fee timeout queue collection.
	VaultFeeTimeoutQueueName = "vault_fee_timeout_queue"
	// VaultPendingSwapOutQueuePrefix is the prefix to retrieve all VaultPendingSwapOutQueue.
	VaultPendingSwapOutQueuePrefix = collections.NewPrefix(3)
	// VaultPendingSwapOutQueueName is a human-readable name for the pending swap out queue collection.
	VaultPendingSwapOutQueueName = "pending_swap_out_queue"
	// VaultPendingSwapOutQueueSeqPrefix is the prefix for the pending swap out queue sequence.
	VaultPendingSwapOutQueueSeqPrefix = collections.NewPrefix(4)
	// VaultPendingSwapOutQueueSeqName is a human-readable name for the pending swap out queue collection.
	VaultPendingSwapOutQueueSeqName = "pending_swap_out_seq"
	// VaultPendingSwapOutByVaultIndexPrefix is the prefix for the pending swap out queue vault index.
	VaultPendingSwapOutByVaultIndexPrefix = collections.NewPrefix(5)
	// VaultPendingSwapOutByVaultIndexName is a human-readable name for the pending swap out queue vault index.
	VaultPendingSwapOutByVaultIndexName = "pending_swap_out_by_vault"
	// VaultPendingSwapOutByIdIndexPrefix is the prefix for the pending swap out queue by id index.
	VaultPendingSwapOutByIdIndexPrefix = collections.NewPrefix(6)
	// VaultPendingSwapOutByIdIndexName is a human-readable name for the pending swap out queue by id index.
	VaultPendingSwapOutByIdIndexName = "pending_swap_out_by_id"
)

// GetVaultAddress returns the module account address for the given shareDenom.
func GetVaultAddress(shareDenom string) sdk.AccAddress {
	return sdk.AccAddress(crypto.AddressHash([]byte(fmt.Sprintf("%s/%s", ModuleName, shareDenom))))
}

// GetProvLabsFeeAddress returns the ProvLabs fee collection address based on the chain ID.
// For pio-mainnet-1, it returns the hardcoded 'pb' address.
// For pio-testnet-1, it returns the hardcoded 'tp' address.
// For any other chain, it returns an address derived from the 'provlabs' byte array.
func GetProvLabsFeeAddress(chainID string) (sdk.AccAddress, error) {
	switch chainID {
	case "pio-mainnet-1":
		addr, err := sdk.AccAddressFromBech32(ProvLabsMainnetFeeAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to parse mainnet fee address: %w", err)
		}
		return addr, nil
	case "pio-testnet-1":
		addr, err := sdk.AccAddressFromBech32(ProvLabsTestnetFeeAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to parse testnet fee address: %w", err)
		}
		return addr, nil
	default:
		return sdk.AccAddress(crypto.AddressHash([]byte("provlabs"))), nil
	}
}
