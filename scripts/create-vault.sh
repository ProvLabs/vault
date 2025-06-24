#!/bin/bash

# --- Fixed Configuration ---
AMOUNT=100 # The amount for the marker is now fixed at 100

# --- Script Dependencies ---
GET_KEY_SCRIPT="./scripts/get-key.sh"
GET_MARKER_ADDR_SCRIPT="./scripts/get-marker-address.sh"
CREATE_MARKER_SCRIPT="./scripts/create-marker.sh"
TX_SCRIPT="./scripts/tx.sh"

# --- Argument Parsing ---
if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]; then
  echo "Usage: $0 <underlying_asset> <share_name> <key_name>"
  echo "Example: $0 hotdogcoin svhotdog myvalidator"
  exit 1
fi

UNDERLYING_ASSET="$1"
SHARE_DENOM="$2"
KEY_NAME="$3"

# Derive ADMIN from the provided key_name (used for vault and as signer for marker creation)
ADMIN=$($GET_KEY_SCRIPT "$KEY_NAME")
if [ -z "$ADMIN" ]; then
  echo "ERROR: Could not retrieve address for key '$KEY_NAME'. Ensure $GET_KEY_SCRIPT works and key exists."
  exit 1
fi

# --- 1. Check if Marker Exists ---
echo "Checking if marker '$UNDERLYING_ASSET' already exists..."
MARKER_ADDR=$($GET_MARKER_ADDR_SCRIPT "$UNDERLYING_ASSET") # Capture output (address or empty)
GET_MARKER_STATUS=$? # Capture exit status (0 for found, 1 for not found)

if [ $GET_MARKER_STATUS -eq 0 ] && [ -n "$MARKER_ADDR" ]; then
  echo "Marker '$UNDERLYING_ASSET' already exists. Skipping creation..."
else
  echo "Marker '$UNDERLYING_ASSET' does not exist or could not be retrieved. Creating it now..."

  # --- 2. Create Marker if it doesn't exist using create-marker.sh ---
  # Pass amount, denom, and key_name directly to create-marker.sh
  CREATE_MARKER_CMD="$CREATE_MARKER_SCRIPT $AMOUNT $UNDERLYING_ASSET $KEY_NAME"
  $CREATE_MARKER_CMD

  # Check if the marker creation was successful based on create-marker.sh's exit code
  if [ $? -ne 0 ]; then
    echo "ERROR: Failed to create marker '$UNDERLYING_ASSET' via create-marker.sh. Aborting."
    exit 1
  fi

  # Immediately query for the newly created marker's address
  MARKER_ADDR=$($GET_MARKER_ADDR_SCRIPT "$UNDERLYING_ASSET")
  GET_CREATED_MARKER_STATUS=$?

  if [ $GET_CREATED_MARKER_STATUS -ne 0 ] || [ -z "$MARKER_ADDR" ]; then
    echo "CRITICAL ERROR: Failed to retrieve address for newly created marker '$UNDERLYING_ASSET'."
    echo "Marker may not have been created correctly or query failed."
    exit 1
  fi
fi

# --- 3. Proceed with vault create-vault ---
if [ -z "$MARKER_ADDR" ]; then
  echo "CRITICAL ERROR: Marker address could not be determined. Cannot create vault."
  exit 1
fi

VAULT_CREATE_CMD="$TX_SCRIPT vault create-vault $ADMIN $UNDERLYING_ASSET $SHARE_DENOM --from $ADMIN"
echo "Executing vault creation: $VAULT_CREATE_CMD"
$VAULT_CREATE_CMD

# Check if the vault creation was successful based on tx.sh's exit code
if [ $? -ne 0 ]; then
  echo "ERROR: Failed to create vault."
  exit 1
fi

echo "Vault creation command sent successfully for $UNDERLYING_ASSET."
