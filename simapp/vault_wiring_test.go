package simapp

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"
)

func TestVaultKeeperProvenanceWiring(t *testing.T) {
	app, _ := setup(t, false, 5, "vaulty-1")

	tests := []struct {
		name  string
		wired any
		want  any
	}{
		{name: "marker keeper", wired: app.VaultKeeper.MarkerKeeper, want: app.MarkerKeeper},
		{name: "metadata keeper", wired: app.VaultKeeper.MetadataKeeper, want: app.MetadataKeeper},
		{name: "name keeper", wired: app.VaultKeeper.NameKeeper, want: app.NameKeeper},
		{name: "attribute keeper", wired: app.VaultKeeper.AttrKeeper, want: app.AttributeKeeper},
		{name: "exchange keeper", wired: app.VaultKeeper.ExchangeKeeper, want: app.ExchangeKeeper},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equalf(t, tt.want, tt.wired, "vault keeper %s should be the app's configured keeper, not the depinject stub", tt.name)
		})
	}

	require.NotNil(t, app.VaultKeeper.ExchangeQueryServer, "vault keeper ExchangeQueryServer should be wired")

	ctx := app.NewContext(true)
	scopeID := metadatatypes.ScopeMetadataAddress(uuid.MustParse("00000000-0000-4000-8000-0000000000aa"))
	var found bool
	require.NotPanics(t, func() {
		_, found = app.VaultKeeper.MetadataKeeper.GetScope(ctx, scopeID)
	}, "scope lookup through the vault keeper's MetadataKeeper should reach a configured store")
	require.Falsef(t, found, "scope %s should not exist in a freshly set up app", scopeID)
}
