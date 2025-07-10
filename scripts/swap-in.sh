#!/bin/bash

# --- Script Dependencies ---
GET_KEY_SCRIPT="./scripts/get-key.sh"
TX_SCRIPT="./scripts/tx.sh"

# --- Argument Parsing ---
if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <amount> <underlying_asset> <vault_address> <key_name>"
  echo "Example: $0 100 hotdogcoin pb18vd8fpwxzck93qlwghaj6arh4p7c5n89x4m9s5 myvalidator"
  exit 1
fi

AMOUNT="$1"
UNDERLYING_ASSET="$2"
VAULT_ADDRESS="$3"
KEY_NAME="$4"

# --- 1. Get Owner Address ---
echo "Retrieving address for key '$KEY_NAME'..."
OWNER=$($GET_KEY_SCRIPT "$KEY_NAME")
if [ -z "$OWNER" ]; then
  echo "ERROR: Could not retrieve address for key '$KEY_NAME'. Ensure the key exists."
  exit 1
fi
echo "Owner address: $OWNER"

# --- 2. Vault Address is now a direct input ---
echo "Using provided vault address: $VAULT_ADDRESS"

# --- 3. Execute Swap In ---
ASSETS="${AMOUNT}${UNDERLYING_ASSET}"
SWAP_IN_CMD="$TX_SCRIPT vault swap-in $OWNER $VAULT_ADDRESS $ASSETS --from $OWNER"
echo "Executing vault swap-in: $SWAP_IN_CMD"
$SWAP_IN_CMD
