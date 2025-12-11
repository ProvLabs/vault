package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PayoutJob holds the details for a pending swap-out to be processed.
type PayoutJob struct {
	Timestamp int64
	ID        uint64
	VaultAddr sdk.AccAddress
	Req       PendingSwapOut
}

// NewPayoutJob creates and returns a new PayoutJob instance.
func NewPayoutJob(timestamp int64, id uint64, vaultAddr sdk.AccAddress, req PendingSwapOut) PayoutJob {
	return PayoutJob{
		Timestamp: timestamp,
		ID:        id,
		VaultAddr: vaultAddr,
		Req:       req,
	}
}
