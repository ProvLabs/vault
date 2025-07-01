#!/bin/bash

# --- Common Binary and Home Directory ---
SIMD_BIN="./simapp/build/simd"
HOME_DIR="--home ./.vaulty" # Ensure this is consistent with your setup

# --- Script Logic ---

# Check if the denom argument was provided
if [ -z "$1" ]; then
  echo "Usage: $0 <denom>"
  echo "Example: $0 hotdogcoin"
  exit 1
fi

# Assign the first argument to the DENOM variable
DENOM="$1"

MARKER_ADDRESS=$($SIMD_BIN q marker get "$DENOM" $HOME_DIR 2>&1 | grep 'base_account' -A 2 | grep 'address' | awk '{ print $2 }' | tr -d '\r')

# Check if an address was successfully extracted
if [ -n "$MARKER_ADDRESS" ]; then
  echo $MARKER_ADDRESS
  exit 0
else
  exit 1
fi

