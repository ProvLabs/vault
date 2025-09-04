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
	// VaultPendingWithdrawalQueuePrefix is the prefix to retrieve all VaultPendingWithdrawalQueue.
	VaultPendingWithdrawalQueuePrefix = collections.NewPrefix(3)
	// VaultPendingWithdrawalQueueName is a human-readable name for the pending withdrawal queue collection.
	VaultPendingWithdrawalQueueName = "pending_withdrawal_queue"
	// VaultPendingWithdrawalQueueSeqPrefix is the prefix for the pending withdrawal queue sequence.
	VaultPendingWithdrawalQueueSeqPrefix = collections.NewPrefix(4)
	// VaultPendingWithdrawalQueueName is a human-readable name for the pending withdrawal queue collection.
	VaultPendingWithdrawalQueueSeqName = "pending_withdrawal_seq"
	// VaultPendingWithdrawalByVaultIndexPrefix is the prefix for the pending withdrawal queue vault index.
	VaultPendingWithdrawalByVaultIndexPrefix = collections.NewPrefix(5)
	// VaultPendingWithdrawalByVaultIndexName is a human-readable name for the pending withdrawal queue vault index.
	VaultPendingWithdrawalByVaultIndexName = "pending_withdrawal_by_vault"
)

// GetVaultAddress returns the module account address for the given shareDenom.
func GetVaultAddress(shareDenom string) sdk.AccAddress {
	return sdk.AccAddress(crypto.AddressHash([]byte(fmt.Sprintf("%s/%s", ModuleName, shareDenom))))
}
