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

func TestProvlabsAddress() Address {
	key := secp256k1.GenPrivKey()
	bytes := key.PubKey().Address().Bytes()

	return Address{
		Bytes:  bytes,
		Bech32: generateAddress("provlabs", bytes),
	}
}

func TestAddress() Address {
	key := secp256k1.GenPrivKey()
	bytes := key.PubKey().Address().Bytes()

	return Address{
		Bytes:  bytes,
		Bech32: generateAddress("cosmos", bytes),
	}
}

func generateAddress(prefix string, bytes []byte) string {
	address, err := sdk.Bech32ifyAddressBytes(prefix, bytes)
	if err != nil {
		panic("error during provlabs address creation")
	}
	return address
}

