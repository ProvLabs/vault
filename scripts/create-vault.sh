#!/bin/bash

# Vault creation is governance-gated: MsgCreateVaultRequest must be signed by the
# gov module account. This script builds that message, wraps it in a governance
# proposal, submits it, votes yes with the given key, and waits for the vault to
# appear. It assumes a local single-node chain started by ./local.sh, where the
# voting period is short and the given key holds all the voting power.

set -uo pipefail

# --- Fixed Configuration ---
AMOUNT=100 # The amount for the marker is now fixed at 100
DEPOSIT="1000000ustake"

# --- Script Dependencies ---
SIMD_BIN="./simapp/build/simd"
GET_KEY_SCRIPT="./scripts/get-key.sh"
GET_MARKER_ADDR_SCRIPT="./scripts/get-marker-address.sh"
CREATE_MARKER_SCRIPT="./scripts/create-marker.sh"
TX_SCRIPT="./scripts/tx.sh"
QUERY_SCRIPT="./scripts/query.sh"

HOME_DIR="--home ./.vaulty"
KEYRING_BACKEND="--keyring-backend test"

# --- Argument Parsing ---
if [ -z "${1:-}" ] || [ -z "${2:-}" ] || [ -z "${3:-}" ]; then
  echo "Usage: $0 <underlying_asset> <share_name> <key_name>"
  echo "Example: $0 hotdogcoin svhotdog myvalidator"
  exit 1
fi

UNDERLYING_ASSET="$1"
SHARE_DENOM="$2"
KEY_NAME="$3"

# Derive ADMIN from the provided key_name. It becomes the vault admin designated by
# the proposal, and signs the marker creation, proposal submission, and vote.
ADMIN=$($GET_KEY_SCRIPT "$KEY_NAME")
if [ -z "$ADMIN" ]; then
  echo "ERROR: Could not retrieve address for key '$KEY_NAME'. Ensure $GET_KEY_SCRIPT works and key exists."
  exit 1
fi

GOV_AUTHORITY=$($SIMD_BIN query auth module-account gov $HOME_DIR --output json 2>/dev/null | jq -r '.account.value.address')
if [ -z "$GOV_AUTHORITY" ] || [ "$GOV_AUTHORITY" = "null" ]; then
  echo "ERROR: Could not resolve the gov module account address. Is the node running?"
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

if [ -z "$MARKER_ADDR" ]; then
  echo "CRITICAL ERROR: Marker address could not be determined. Cannot create vault."
  exit 1
fi

# --- 3. Build the governance proposal containing MsgCreateVaultRequest ---
PROPOSAL_FILE=$(mktemp -t create-vault-proposal)
trap 'rm -f "$PROPOSAL_FILE"' EXIT

cat > "$PROPOSAL_FILE" <<EOF
{
  "messages": [
    {
      "@type": "/provlabs.vault.v1.MsgCreateVaultRequest",
      "authority": "$GOV_AUTHORITY",
      "admin": "$ADMIN",
      "share_denom": "$SHARE_DENOM",
      "underlying_asset": "$UNDERLYING_ASSET",
      "withdrawal_delay_seconds": "0"
    }
  ],
  "metadata": "",
  "deposit": "$DEPOSIT",
  "title": "Create vault $SHARE_DENOM",
  "summary": "Create a vault with share denom $SHARE_DENOM backed by $UNDERLYING_ASSET, administered by $ADMIN."
}
EOF

echo "Submitting governance proposal to create vault '$SHARE_DENOM'..."
$TX_SCRIPT gov submit-proposal "$PROPOSAL_FILE" --from "$ADMIN"
if [ $? -ne 0 ]; then
  echo "ERROR: Failed to submit the vault creation proposal."
  exit 1
fi

# --- 4. Vote on the proposal ---
PROPOSAL_ID=$($SIMD_BIN query gov proposals $HOME_DIR --output json 2>/dev/null | jq -r '.proposals | max_by(.id | tonumber) | .id')
if [ -z "$PROPOSAL_ID" ] || [ "$PROPOSAL_ID" = "null" ]; then
  echo "ERROR: Could not determine the submitted proposal id."
  exit 1
fi

echo "Voting yes on proposal $PROPOSAL_ID..."
$TX_SCRIPT gov vote "$PROPOSAL_ID" yes --from "$ADMIN"
if [ $? -ne 0 ]; then
  echo "ERROR: Failed to vote on proposal $PROPOSAL_ID."
  exit 1
fi

# --- 5. Wait for the proposal to pass and the vault to exist ---
echo "Waiting for proposal $PROPOSAL_ID to pass..."
for _ in $(seq 1 60); do
  STATUS=$($SIMD_BIN query gov proposal "$PROPOSAL_ID" $HOME_DIR --output json 2>/dev/null | jq -r '.proposal.status')
  case "$STATUS" in
    PROPOSAL_STATUS_PASSED)
      echo "Proposal $PROPOSAL_ID passed. Vault '$SHARE_DENOM' created."
      $QUERY_SCRIPT vault get "$SHARE_DENOM"
      exit 0
      ;;
    PROPOSAL_STATUS_REJECTED|PROPOSAL_STATUS_FAILED)
      echo "ERROR: Proposal $PROPOSAL_ID ended with status $STATUS."
      exit 1
      ;;
  esac
  sleep 2
done

echo "ERROR: Timed out waiting for proposal $PROPOSAL_ID to complete. Last status: ${STATUS:-unknown}."
exit 1
