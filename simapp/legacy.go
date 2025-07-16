package simapp

import (
	storetypes "cosmossdk.io/store/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/ibc-go/modules/capability"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
	solomachine "github.com/cosmos/ibc-go/v8/modules/light-clients/06-solomachine"
	tendermint "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
)

func (app *SimApp) RegisterLegacyModules() error {
	if err := app.RegisterStores(
		storetypes.NewKVStoreKey(capabilitytypes.StoreKey),
		storetypes.NewMemoryStoreKey(capabilitytypes.MemStoreKey),
		storetypes.NewKVStoreKey(exported.StoreKey),
	); err != nil {
		return err
	}

	app.ParamsKeeper.Subspace(exported.ModuleName).WithKeyTable(clienttypes.ParamKeyTable().RegisterParamSet(&connectiontypes.Params{}))

	app.CapabilityKeeper = capabilitykeeper.NewKeeper(
		app.appCodec,
		app.GetKey(capabilitytypes.StoreKey),
		app.GetMemKey(capabilitytypes.MemStoreKey),
	)

	scopedIBCKeeper := app.CapabilityKeeper.ScopeToModule(exported.ModuleName)
	app.IBCKeeper = ibckeeper.NewKeeper(
		app.appCodec,
		app.GetKey(exported.StoreKey),
		app.GetSubspace(exported.ModuleName),
		app.StakingKeeper,
		app.UpgradeKeeper,
		scopedIBCKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	return app.RegisterModules(
		capability.NewAppModule(app.appCodec, *app.CapabilityKeeper, true),
		ibc.NewAppModule(app.IBCKeeper),
		tendermint.NewAppModule(),
		solomachine.NewAppModule(),
	)
}
