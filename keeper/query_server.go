package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/provlabs/vault/types"
)

var _ types.QueryServer = &queryServer{}

type queryServer struct {
	*Keeper
}

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
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "vault with address %q not found", req.VaultAddress)
	}

	return &types.QueryVaultResponse{
		Vault: *vault,
	}, nil
}

// TotalAssets returns the total amount of the underlying asset managed by the vault.
func (k queryServer) TotalAssets(goCtx context.Context, req *types.QueryTotalAssetsRequest) (*types.QueryTotalAssetsResponse, error) {
	panic("not implemented")
}

//TODO possibly add a ShareHolders query to query holders of the share denom or have that part of TotalAssets?

// Params returns the params for the module.
func (q queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	panic("not implemented")
}
