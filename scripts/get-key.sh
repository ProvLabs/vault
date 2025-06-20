#!/bin/bash

KEY=$1
SIMD_BIN="./simapp/build/simd"
KEYRING_BACKEND="--keyring-backend test"
HOME_DIR="--home ./.vaulty"
CHAIN_ID="--chain-id vaulty-1"
YES_FLAG="--yes"

KEY_ADDR=$($SIMD_BIN keys list $HOME_DIR $KEYRING_BACKEND | grep "name: $KEY" -B 1 | grep 'address:' | awk '{ print $3 }' | tr -d '\r')

# Check if an address was successfully extracted
if [ -n "$KEY_ADDR" ]; then
  echo $KEY_ADDR
  exit 0
else
  exit 1
fi
