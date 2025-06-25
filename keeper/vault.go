package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provlabs/vault/types"
)

const (
	Supply          = 0
	NoFixedSupply   = false
	NoForceTransfer = false
	NoGovControl    = false
)

// CreateVaultMarker creates, finalizes, and activates a new restricted marker for the vault's share denomination.
func (k *Keeper) CreateVaultMarker(ctx sdk.Context, admin, shareDenom, underlyingAsset string) (*markertypes.MarkerAccount, error) {
	markerManager := authtypes.NewModuleAddress(types.ModuleName)

	vaultShareMarkerAddress := markertypes.MustGetMarkerAddress(shareDenom)
	if found := k.MarkerKeeper.IsMarkerAccount(ctx, vaultShareMarkerAddress); found {
		return nil, fmt.Errorf("a marker with the share denomination %q already exists", shareDenom)
	}

	baseAccount := authtypes.NewBaseAccountWithAddress(vaultShareMarkerAddress)
	newMarker := markertypes.NewMarkerAccount(
		baseAccount,
		sdk.NewInt64Coin(shareDenom, Supply),
		markerManager,
		[]markertypes.AccessGrant{
			{
				Address: markerManager.String(),
				Permissions: []markertypes.Access{
					markertypes.Access_Admin,
					markertypes.Access_Mint,
					markertypes.Access_Burn,
					markertypes.Access_Withdraw,
					markertypes.Access_Transfer,
				},
			},
		},
		markertypes.StatusProposed,
		markertypes.MarkerType_RestrictedCoin,
		NoFixedSupply,
		NoGovControl,
		NoForceTransfer,
		[]string{},
	)

	if err := k.MarkerKeeper.AddFinalizeAndActivateMarker(ctx, newMarker); err != nil {
		return nil, fmt.Errorf("failed to create and activate vault share marker: %w", err)
	}

	return newMarker, nil
}
