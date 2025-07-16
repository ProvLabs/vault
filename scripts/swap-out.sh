#!/bin/bash

# --- Script Dependencies ---
GET_KEY_SCRIPT="./scripts/get-key.sh"
TX_SCRIPT="./scripts/tx.sh"

# --- Argument Parsing ---
if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <amount> <share_denom> <vault_address> <key_name>"
  echo "Example: $0 50 svhotdog pb18vd8fpwxzck93qlwghaj6arh4p7c5n89x4m9s5 myvalidator"
  exit 1
fi

AMOUNT="$1"
SHARE_DENOM="$2"
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

# --- 2. Execute Swap Out ---
SHARES_TO_BURN="${AMOUNT}${SHARE_DENOM}"
SWAP_OUT_CMD="$TX_SCRIPT vault swap-out $OWNER $VAULT_ADDRESS $SHARES_TO_BURN --from $OWNER"
echo "Executing vault swap-out: $SWAP_OUT_CMD"
$SWAP_OUT_CMD
