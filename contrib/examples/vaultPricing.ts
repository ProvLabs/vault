/**
 * vaultPricing.ts
 *
 * Pulls pricing information for a single Provenance vault using the x/vault
 * module's `Query/Vault` gRPC-gateway (REST) endpoint:
 *
 *   GET {LCD}/vault/v1/vaults/{id}
 *
 * `id` may be either the vault's bech32 address or its share denom.
 *
 * The response carries everything needed to derive a share price:
 *   - vault.total_shares      canonical supply of shares (including bridged shares)
 *   - vault.underlying_asset  the denom every value below is expressed in
 *   - total_vault_value       principal + accrued-but-unpaid interest, excluding reserves
 *   - principal / reserves    raw balances on the share marker and the vault account
 *
 * NAV per share = total_vault_value.amount / total_shares.amount
 *
 * Amounts on the wire are integer strings in the denom's smallest unit, so the math
 * below uses BigInt and a fixed-point scale to avoid floating point loss.
 *
 * Usage:
 *   npx tsx vaultPricing.ts <vault-address-or-share-denom> [lcd-url]
 *
 * Requires Node 18+ (global fetch). No third-party dependencies.
 */

// ---------------------------------------------------------------------------
// Wire types (JSON shape produced by the gRPC-gateway; field names are snake_case)
// ---------------------------------------------------------------------------

/** cosmos.base.v1beta1.Coin */
interface Coin {
  denom: string;
  amount: string;
}

/** provlabs.vault.v1.AccountBalance */
interface AccountBalance {
  address: string;
  coins: Coin[];
}

/** cosmos.auth.v1beta1.BaseAccount (embedded into VaultAccount) */
interface BaseAccount {
  address: string;
  pub_key: unknown | null;
  account_number: string;
  sequence: string;
}

/**
 * provlabs.vault.v1.VaultAccount, trimmed to the fields relevant to pricing.
 * The endpoint returns more fields than are listed here; extras are ignored.
 */
interface VaultAccount {
  base_account: BaseAccount;
  total_shares: Coin;
  underlying_asset: string;
  admin: string;
  current_interest_rate: string;
  desired_interest_rate: string;
  period_start: string;
  period_timeout: string;
  swap_in_enabled: boolean;
  swap_out_enabled: boolean;
  withdrawal_delay_seconds: string;
  paused: boolean;
  paused_balance: Coin;
  paused_reason: string;
}

/** provlabs.vault.v1.QueryVaultResponse */
interface QueryVaultResponse {
  vault: VaultAccount;
  principal: AccountBalance;
  reserves: AccountBalance;
  total_vault_value: Coin;
}

/** Error body returned by the gRPC-gateway on a non-2xx response. */
interface GatewayError {
  code: number;
  message: string;
  details: unknown[];
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

/** Public Provenance LCD (REST) endpoints. Override with the second CLI argument. */
const DEFAULT_LCD_URL = "https://api.provenance.io";

/** Decimal places used for the fixed-point NAV-per-share calculation. */
const PRICE_PRECISION = 18;

/**
 * Fetches the raw `QueryVaultResponse` for a vault.
 *
 * @param id     bech32 vault address or share denom
 * @param lcdUrl base URL of a Provenance LCD/REST node
 */
export async function queryVault(id: string, lcdUrl: string = DEFAULT_LCD_URL): Promise<QueryVaultResponse> {
  const url = `${lcdUrl.replace(/\/$/, "")}/vault/v1/vaults/${encodeURIComponent(id)}`;

  const res = await fetch(url, { headers: { accept: "application/json" } });
  if (!res.ok) {
    let detail = res.statusText;
    try {
      const body = (await res.json()) as GatewayError;
      detail = `${body.message} (grpc code ${body.code})`;
    } catch {
      // body was not JSON; fall back to the HTTP status text
    }
    throw new Error(`failed to query vault ${id} at ${url}: HTTP ${res.status} ${detail}`);
  }

  return (await res.json()) as QueryVaultResponse;
}

// ---------------------------------------------------------------------------
// Pricing
// ---------------------------------------------------------------------------

/** Derived pricing figures for a vault, all denominated in the vault's underlying asset. */
export interface VaultPricing {
  vaultAddress: string;
  shareDenom: string;
  underlyingAsset: string;

  /** Total shares outstanding, smallest unit. */
  totalShares: bigint;
  /** Estimated total vault value (principal + accrued interest, excluding reserves), smallest unit. */
  totalVaultValue: bigint;
  /** Underlying asset actually sitting on the share marker, smallest unit. */
  principalBalance: bigint;
  /** Underlying asset held in the vault account to fund positive interest, smallest unit. */
  reserveBalance: bigint;

  /** total_vault_value / total_shares, as a decimal string with PRICE_PRECISION places. */
  navPerShare: string;

  currentInterestRate: string;
  desiredInterestRate: string;
  paused: boolean;
  swapInEnabled: boolean;
  swapOutEnabled: boolean;
}

/** Returns the amount of `denom` in a coin list, or 0n when absent. */
function balanceOf(coins: Coin[], denom: string): bigint {
  const coin = coins.find((c) => c.denom === denom);
  return coin ? BigInt(coin.amount) : 0n;
}

/**
 * Divides two BigInts and renders the quotient as a decimal string with `precision` places.
 * Truncates (floors) rather than rounds, matching the module's conservative share math.
 */
function divideToDecimalString(numerator: bigint, denominator: bigint, precision: number): string {
  if (denominator === 0n) {
    return "0";
  }
  const scale = 10n ** BigInt(precision);
  const scaled = (numerator * scale) / denominator;
  const whole = scaled / scale;
  const frac = (scaled % scale).toString().padStart(precision, "0");
  return `${whole}.${frac}`;
}

/** Converts a `QueryVaultResponse` into human-friendly pricing figures. */
export function derivePricing(resp: QueryVaultResponse): VaultPricing {
  const { vault, principal, reserves, total_vault_value } = resp;

  const totalShares = BigInt(vault.total_shares.amount);
  const totalVaultValue = BigInt(total_vault_value.amount);

  return {
    vaultAddress: vault.base_account.address,
    shareDenom: vault.total_shares.denom,
    underlyingAsset: vault.underlying_asset,

    totalShares,
    totalVaultValue,
    principalBalance: balanceOf(principal.coins, vault.underlying_asset),
    reserveBalance: balanceOf(reserves.coins, vault.underlying_asset),

    navPerShare: divideToDecimalString(totalVaultValue, totalShares, PRICE_PRECISION),

    currentInterestRate: vault.current_interest_rate,
    desiredInterestRate: vault.desired_interest_rate,
    paused: vault.paused,
    swapInEnabled: vault.swap_in_enabled,
    swapOutEnabled: vault.swap_out_enabled,
  };
}

/**
 * Convenience wrapper: query a vault and return its derived pricing in one call.
 */
export async function getVaultPricing(id: string, lcdUrl: string = DEFAULT_LCD_URL): Promise<VaultPricing> {
  const resp = await queryVault(id, lcdUrl);
  return derivePricing(resp);
}

// ---------------------------------------------------------------------------
// CLI entry point
// ---------------------------------------------------------------------------

async function main(): Promise<void> {
  const [id, lcdUrl] = process.argv.slice(2);
  if (!id) {
    console.error("usage: npx tsx vaultPricing.ts <vault-address-or-share-denom> [lcd-url]");
    process.exit(1);
  }

  const pricing = await getVaultPricing(id, lcdUrl);

  console.log(`Vault:              ${pricing.vaultAddress}`);
  console.log(`Share denom:        ${pricing.shareDenom}`);
  console.log(`Underlying asset:   ${pricing.underlyingAsset}`);
  console.log(`Total shares:       ${pricing.totalShares}`);
  console.log(`Total vault value:  ${pricing.totalVaultValue} ${pricing.underlyingAsset}`);
  console.log(`Principal balance:  ${pricing.principalBalance} ${pricing.underlyingAsset}`);
  console.log(`Reserve balance:    ${pricing.reserveBalance} ${pricing.underlyingAsset}`);
  console.log(`NAV per share:      ${pricing.navPerShare} ${pricing.underlyingAsset}/${pricing.shareDenom}`);
  console.log(`Interest rate:      current=${pricing.currentInterestRate} desired=${pricing.desiredInterestRate}`);
  console.log(`Status:             paused=${pricing.paused} swapIn=${pricing.swapInEnabled} swapOut=${pricing.swapOutEnabled}`);
}

// Only run the CLI when executed directly, so the exports can be imported elsewhere.
const isDirectRun = process.argv[1] !== undefined && import.meta.url === new URL(`file://${process.argv[1]}`).href;
if (isDirectRun) {
  main().catch((err) => {
    console.error(err instanceof Error ? err.message : err);
    process.exit(1);
  });
}
