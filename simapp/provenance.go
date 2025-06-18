package simapp

import (
	storetypes "cosmossdk.io/store/types"
	"github.com/provenance-io/provenance/x/attribute"
	attributekeeper "github.com/provenance-io/provenance/x/attribute/keeper"
	attributetypes "github.com/provenance-io/provenance/x/attribute/types"
	"github.com/provenance-io/provenance/x/marker"
	markerkeeper "github.com/provenance-io/provenance/x/marker/keeper"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
	"github.com/provenance-io/provenance/x/name"
	namekeeper "github.com/provenance-io/provenance/x/name/keeper"
	nametypes "github.com/provenance-io/provenance/x/name/types"
)

func (app *SimApp) RegisterProvenanceModules() error {
	if err := app.RegisterStores(
		storetypes.NewKVStoreKey(markertypes.StoreKey),
		storetypes.NewKVStoreKey(attributetypes.StoreKey),
		storetypes.NewKVStoreKey(nametypes.StoreKey),
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

	return app.RegisterModules(
		name.NewAppModule(app.appCodec, app.NameKeeper, app.AccountKeeper, app.BankKeeper),
		attribute.NewAppModule(app.appCodec, app.AttributeKeeper, app.AccountKeeper, app.BankKeeper, app.NameKeeper),
		marker.NewAppModule(app.appCodec, app.MarkerKeeper, app.AccountKeeper, app.BankKeeper, app.FeegrantKeeper, app.GovKeeper, app.AttributeKeeper, app.interfaceRegistry),
	)
}
