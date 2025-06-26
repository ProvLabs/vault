package utils

import (
	"crypto/rand"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func GenerateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

type Address struct {
	Bytes  []byte
	Bech32 string
}

func TestAddress() Address {
	key := secp256k1.GenPrivKey()
	bytes := key.PubKey().Address().Bytes()

	return Address{
		Bytes:  bytes,
		Bech32: generateProvlabsAddress(bytes),
	}
}

func generateProvlabsAddress(bytes []byte) string {
	address, err := sdk.Bech32ifyAddressBytes("provlabs", bytes)
	if err != nil {
		panic("error during cosmos address creation")
	}
	return address
}
