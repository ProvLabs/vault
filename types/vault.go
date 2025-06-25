package types

// NewVault creates a new vault.
func NewVault(admin, address, underlyingAsset string) *Vault {
	return &Vault{
		Admin:           admin,
		VaultAddress:    address,
		UnderlyingAsset: underlyingAsset,
	}
}
