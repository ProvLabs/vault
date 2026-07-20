package simapp

import (
	storetypes "cosmossdk.io/store/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/provenance-io/provenance/x/attribute"
	attributekeeper "github.com/provenance-io/provenance/x/attribute/keeper"
	attributetypes "github.com/provenance-io/provenance/x/attribute/types"
	"github.com/provenance-io/provenance/x/exchange"
	exchangekeeper "github.com/provenance-io/provenance/x/exchange/keeper"
	exchangemodule "github.com/provenance-io/provenance/x/exchange/module"
	"github.com/provenance-io/provenance/x/hold"
	holdkeeper "github.com/provenance-io/provenance/x/hold/keeper"
	holdmodule "github.com/provenance-io/provenance/x/hold/module"
	"github.com/provenance-io/provenance/x/marker"
	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provenance-io/provenance/x/metadata"
	metadatakeeper "github.com/provenance-io/provenance/x/metadata/keeper"
	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"
	"github.com/provenance-io/provenance/x/name"
	namekeeper "github.com/provenance-io/provenance/x/name/keeper"
	nametypes "github.com/provenance-io/provenance/x/name/types"
)

// RegisterProvenanceModules sets up and registers the Provenance modules
// used by the SimApp, including name, attribute, marker, and vault modules.
//
// It performs the following actions:
//   - Registers the KV store keys required by the name, attribute, and marker modules.
//   - Initializes the NameKeeper, AttributeKeeper, and MarkerKeeper using the legacy Provenance wiring pattern.
//   - Injects the MarkerKeeper, NameKeeper, and AttributeKeeper into the VaultKeeper (via app.VaultKeeper.MarkerKeeper,
//     app.VaultKeeper.NameKeeper, and app.VaultKeeper.AttrKeeper) to enable restricted marker management,
//     name resolution, and attribute-based gating within the vault module.
//   - Registers the modules with the app for inclusion in BeginBlocker, EndBlocker, InitGenesis, etc.
//
// This function is typically called during app initialization to ensure
// all Provenance modules are correctly configured and available.
//
// Returns an error if store registration fails.
func (app *SimApp) RegisterProvenanceModules() error {
	if err := app.RegisterStores(
		storetypes.NewKVStoreKey(markertypes.StoreKey),
		storetypes.NewKVStoreKey(attributetypes.StoreKey),
		storetypes.NewKVStoreKey(nametypes.StoreKey),
		storetypes.NewKVStoreKey(metadatatypes.StoreKey),
		storetypes.NewKVStoreKey(hold.StoreKey),
		storetypes.NewKVStoreKey(exchange.StoreKey),
	); err != nil {
		return err
	}

	app.NameKeeper = namekeeper.NewKeeper(
		app.appCodec,
		app.GetKey(nametypes.StoreKey),
	)

	app.AttributeKeeper = attributekeeper.NewKeeper(
		app.appCodec,
		app.GetKey(attributetypes.StoreKey),
		app.AccountKeeper,
		&app.NameKeeper,
	)

	app.MarkerKeeper = markerkeeper.NewKeeper(
		app.appCodec,
		app.GetKey(markertypes.StoreKey),
		app.AccountKeeper,
		app.BankKeeper,
		app.AuthzKeeper,
		app.FeegrantKeeper,
		app.AttributeKeeper,
		app.NameKeeper,
		nil,
		nil,
		NewGroupCheckerFunc(app.GroupKeeper),
	)

	app.MetadataKeeper = metadatakeeper.NewKeeper(
		app.appCodec,
		app.GetKey(metadatatypes.StoreKey),
		app.AccountKeeper,
		app.AuthzKeeper,
		app.AttributeKeeper,
		app.MarkerKeeper,
		app.BankKeeper,
	)

	app.HoldKeeper = holdkeeper.NewKeeper(
		app.appCodec,
		app.GetKey(hold.StoreKey),
		app.AccountKeeper,
		app.BankKeeper,
	)

	app.ExchangeKeeper = exchangekeeper.NewKeeper(
		app.appCodec,
		app.GetKey(exchange.StoreKey),
		authtypes.FeeCollectorName,
		app.AccountKeeper,
		app.AttributeKeeper,
		app.BankKeeper,
		app.HoldKeeper,
		app.MarkerKeeper,
		app.MetadataKeeper,
	)

	app.VaultKeeper.MarkerKeeper = app.MarkerKeeper
	app.VaultKeeper.NameKeeper = app.NameKeeper
	app.VaultKeeper.AttrKeeper = app.AttributeKeeper
	app.VaultKeeper.ExchangeKeeper = app.ExchangeKeeper
	app.VaultKeeper.ExchangeQueryServer = exchangekeeper.NewQueryServer(app.ExchangeKeeper)

	return app.RegisterModules(
		name.NewAppModule(app.appCodec, app.NameKeeper, app.AccountKeeper, app.BankKeeper),
		attribute.NewAppModule(app.appCodec, app.AttributeKeeper, app.AccountKeeper, app.BankKeeper, app.NameKeeper),
		marker.NewAppModule(app.appCodec, app.MarkerKeeper, app.AccountKeeper, app.BankKeeper, app.FeegrantKeeper, *app.GovKeeper, app.AttributeKeeper, app.interfaceRegistry),
		metadata.NewAppModule(app.appCodec, app.MetadataKeeper, app.AccountKeeper),
		holdmodule.NewAppModule(app.appCodec, app.HoldKeeper),
		exchangemodule.NewAppModule(app.appCodec, app.ExchangeKeeper),
	)
}
