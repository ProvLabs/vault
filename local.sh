#!/bin/bash

set -euo pipefail

SIMD_BIN=./simapp/build/simd
CHAIN_ID="${CHAIN_ID:-vaulty-1}"

for arg in "$@"
do
    case $arg in
        -r|--reset)
        rm -rf .vaulty
        shift
        ;;
    esac
done

if ! [ -f .vaulty/data/priv_validator_state.json ]; then
  "$SIMD_BIN" init validator --chain-id "$CHAIN_ID" --home .vaulty &> /dev/null
  "$SIMD_BIN" keys add account-1 --home .vaulty --keyring-backend test --recover <<< "picture pigeon chicken cause quarter script snow concert slim pill wedding oval vacant crew kingdom music ghost symbol sport poet velvet enhance warfare diet" &> /dev/null
  "$SIMD_BIN" keys add account-2 --home .vaulty --keyring-backend test --recover <<< "coyote online awkward talent share next hamster speed minimum color false amazing balcony treat squirrel feature combine negative fame decide usage mistake nurse begin" &> /dev/null
  "$SIMD_BIN" keys add validator --home .vaulty --keyring-backend test &> /dev/null
  "$SIMD_BIN" genesis add-genesis-account validator 1000000ustake --home .vaulty --keyring-backend test
  "$SIMD_BIN" genesis add-genesis-account provlabs19ftpcggezgal5ascglq5m022z4e453khv4j3k2 1000000uusdc --home .vaulty
  "$SIMD_BIN" genesis add-genesis-account provlabs1evyv7neax9qtxxzuexnhylxyz4guvsyjhxyv47 1000000uusdc --home .vaulty

  TEMP=.vaulty/genesis.json
  touch $TEMP && jq '.app_state.staking.params.bond_denom = "ustake"' .vaulty/config/genesis.json > $TEMP && mv $TEMP .vaulty/config/genesis.json

  "$SIMD_BIN" genesis gentx validator 1000000ustake --chain-id "$CHAIN_ID" --home .vaulty --keyring-backend test &> /dev/null
  "$SIMD_BIN" genesis collect-gentxs --home .vaulty &> /dev/null 

  sed -i '' 's/timeout_commit = "5s"/timeout_commit = "1s"/g' .vaulty/config/config.toml
fi

"$SIMD_BIN" start --home .vaulty
