#!/bin/bash

# --- Fixed Configuration (if any) ---
# SUPPLY and GOV are still fixed unless you want them as arguments too
SUPPLY="--supplyFixed=false"
GOV="--allowGovernanceControl=false"
TX=./scripts/tx.sh # Path to your tx.sh script
PERMISSIONS="mint,burn,withdraw,admin" # Default permissions for the new marker

# --- Check for arguments ---
if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]; then
  echo "Usage: $0 <amount> <asset_name> <key_name>"
  echo "Example: $0 1000 hotdogcoin myvalidator"
  exit 1
fi

# --- Assign command-line arguments to variables ---
AMOUNT="$1"
ASSET="$2"
KEY_NAME="$3" # New: Third argument is the key name

# Get the signer's address from the provided key_name
SIGNER=$(./scripts/get-key.sh "$KEY_NAME")
if [ -z "$SIGNER" ]; then
  echo "ERROR: Could not retrieve address for key: '$KEY_NAME'"
  exit 1
fi
FROM="--from $SIGNER" # Construct the --from flag with the dynamic signer

# --- Execute the command ---
CMD="$TX marker create-finalize-activate ${AMOUNT}${ASSET} $SIGNER,$PERMISSIONS $SUPPLY $GOV $FROM"
$CMD
