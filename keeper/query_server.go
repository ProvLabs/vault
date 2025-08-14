package keeper

import (
	"context"
	"fmt"

	"github.com/provlabs/vault/types"
	"github.com/provlabs/vault/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

var _ types.QueryServer = &queryServer{}

type queryServer struct {
	*Keeper
}

// NewQueryServer creates a new QueryServer for the module.
func NewQueryServer(keeper *Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

// Vaults returns a paginated list of all vaults.
func (k queryServer) Vaults(goCtx context.Context, req *types.QueryVaultsRequest) (*types.QueryVaultsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vaults := []types.VaultAccount{}

	_, pageRes, err := query.CollectionFilteredPaginate(
		ctx,
		k.Keeper.Vaults,
		req.Pagination,
		func(key sdk.AccAddress, val []byte) (include bool, err error) {
			vault, _ := k.GetVault(ctx, key)
			vaults = append(vaults, *vault)
			return true, nil
		},
		func(_ sdk.AccAddress, value []byte) ([]byte, error) {
			return value, nil
		},
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryVaultsResponse{
		Vaults:     vaults,
		Pagination: pageRes,
	}, nil
}

// Vault returns the configuration and state of a specific vault.
func (k queryServer) Vault(goCtx context.Context, req *types.QueryVaultRequest) (*types.QueryVaultResponse, error) {
	if req == nil || req.VaultAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "vault_address must be provided")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid vault_address: %v", err)
	}

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil || vault == nil {
		return nil, status.Errorf(codes.NotFound, "vault with address %q not found", req.VaultAddress)
	}

	return &types.QueryVaultResponse{
		Vault: *vault,
	}, nil
}

// EstimateSwapIn estimates the amount of shares that would be received for a given amount of underlying assets.
func (k queryServer) EstimateSwapIn(goCtx context.Context, req *types.QueryEstimateSwapInRequest) (*types.QueryEstimateSwapInResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.VaultAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "vault_address must be provided")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid vault_address: %v", err)
	}

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil || vault == nil {
		return nil, status.Errorf(codes.NotFound, "vault with address %q not found", req.VaultAddress)
	}

	if err := vault.ValidateUnderlyingAssets(req.Assets); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid asset for vault: %v", err)
	}

	markerAddr := markertypes.MustGetMarkerAddress(vault.ShareDenom)
	totalShares := k.BankKeeper.GetSupply(ctx, vault.ShareDenom).Amount
	totalAssets := k.BankKeeper.GetBalance(ctx, markerAddr, vault.UnderlyingAssets[0])

	estimatedTotalAssets, err := k.CalculateVaultTotalAssets(ctx, vault, totalAssets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to estimate total assets: %v", err)
	}

	estimatedShares, err := utils.CalculateSharesFromAssets(req.Assets.Amount, estimatedTotalAssets, totalShares, vault.ShareDenom)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate shares from assets: %w", err)
	}

	return &types.QueryEstimateSwapInResponse{
		Assets: estimatedShares,
		Height: ctx.BlockHeight(),
		Time:   ctx.BlockTime().UTC(),
	}, nil
}

// EstimateSwapOut estimates the amount of underlying assets that would be received for a given amount of shares.
func (k queryServer) EstimateSwapOut(goCtx context.Context, req *types.QueryEstimateSwapOutRequest) (*types.QueryEstimateSwapOutResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.VaultAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "vault_address must be provided")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	vaultAddr, err := sdk.AccAddressFromBech32(req.VaultAddress)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid vault_address: %v", err)
	}

	vault, err := k.GetVault(ctx, vaultAddr)
	if err != nil || vault == nil {
		return nil, status.Errorf(codes.NotFound, "vault with address %q not found", req.VaultAddress)
	}

	if req.Assets.Denom != vault.ShareDenom {
		return nil, status.Errorf(codes.InvalidArgument, "asset denom %s does not match vault share denom %s", req.Assets.Denom, vault.ShareDenom)
	}

	markerAddr := markertypes.MustGetMarkerAddress(vault.ShareDenom)
	totalShares := k.BankKeeper.GetSupply(ctx, vault.ShareDenom).Amount
	totalAssets := k.BankKeeper.GetBalance(ctx, markerAddr, vault.UnderlyingAssets[0])

	estimatedTotalAssets, err := k.CalculateVaultTotalAssets(ctx, vault, totalAssets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to estimate total assets: %v", err)
	}

	estimatedAssets, err := utils.CalculateAssetsFromShares(req.Assets.Amount, totalShares, estimatedTotalAssets, vault.UnderlyingAssets[0])
	if err != nil {
		return nil, fmt.Errorf("failed to calculate assets from shares: %w", err)
	}

	return &types.QueryEstimateSwapOutResponse{
		Assets: estimatedAssets,
		Height: ctx.BlockHeight(),
		Time:   ctx.BlockTime().UTC(),
	}, nil
}
