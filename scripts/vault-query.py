#!/usr/bin/env python3
import argparse
import json
import sys
import time
from datetime import datetime, timezone
from decimal import Decimal, getcontext, ROUND_DOWN
from urllib.parse import urlencode, quote
from urllib.request import Request, urlopen

NETWORK_DEFAULTS = {
    "": "https://pl-testnet-1-api.test.provlabs.com:443",
    "pl-testnet": "https://pl-testnet-1-api.test.provlabs.com:443",
    "pio-mainnet": "https://api.provenance.io:443",
    "pio-testnet": "https://api.test.provenance.io:443",
}

SECONDS_PER_YEAR = Decimal("31536000")

def node_from_network(key: str) -> str:
    if key in NETWORK_DEFAULTS:
        return NETWORK_DEFAULTS[key]
    if key.startswith("http://") or key.startswith("https://"):
        return key
    sys.stderr.write(f"Unknown network key: {key}\n")
    sys.exit(2)

def http_get_json(url: str, headers: dict | None = None) -> dict:
    req = Request(url, headers=headers or {})
    with urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))

def parse_rfc3339_to_unix(s: str) -> int:
    dt = datetime.fromisoformat(s.replace("Z", "+00:00"))
    return int(dt.replace(tzinfo=timezone.utc).timestamp())

def chain_now_ts(base: str) -> int:
    url = f"{base}/cosmos/base/tendermint/v1beta1/blocks/latest"
    data = http_get_json(url)
    t = (
        data.get("block", {})
            .get("header", {})
            .get("time")
    )
    if not t:
        return int(time.time())
    return parse_rfc3339_to_unix(t)

def dt_utc_str(ts: int | None) -> str | None:
    if not ts or ts <= 0:
        return None
    return datetime.fromtimestamp(ts, tz=timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")

def as_int(x, default=0) -> int:
    try:
        if x is None:
            return default
        if isinstance(x, int):
            return x
        if isinstance(x, str) and x.strip() == "":
            return default
        return int(x)
    except Exception:
        return default

def as_str(x, default="") -> str:
    if x is None:
        return default
    return str(x)

def dec_floor(n: Decimal) -> int:
    return int(n.to_integral_value(rounding=ROUND_DOWN))

# calculate total vault value with continuous compounding interest
def calc_tvv_continuous(P_int: int, rate_str: str, elapsed_sec: int) -> int:
    if P_int <= 0:
        return 0
    r = Decimal(rate_str)
    if r <= 0 or elapsed_sec <= 0:
        return P_int
    t = Decimal(elapsed_sec) / SECONDS_PER_YEAR
    A = Decimal(P_int) * (r * t).exp()
    interest = A - Decimal(P_int)
    interest_int = dec_floor(interest)
    return P_int + interest_int

def clamp(v: int, lo: int | None, hi: int | None) -> int:
    if lo is not None and v < lo:
        return lo
    if hi is not None and hi > 0 and v > hi:
        return hi
    return v

def build_report(node: str, j: dict, now_ts: int) -> dict:
    getcontext().prec = 60
    vault_addr = j.get("vault", {}).get("base_account", {}).get("address")
    share_denom = j.get("vault", {}).get("total_shares", {}).get("denom")
    total_shares = as_int(j.get("vault", {}).get("total_shares", {}).get("amount"), 0)
    underlying = j.get("vault", {}).get("underlying_asset")
    start = as_int(j.get("vault", {}).get("period_start"), 0)
    timeout = as_int(j.get("vault", {}).get("period_timeout"), 0)
    rate_str = as_str(j.get("vault", {}).get("current_interest_rate"))
    principal_list = j.get("principal", {}).get("coins", []) or []
    principal_amt = 0
    for c in principal_list:
        if c.get("denom") == underlying:
            principal_amt = as_int(c.get("amount"), 0)
            break
    reported = as_int(j.get("total_vault_value", {}).get("amount"), 0)
    per_share_underlying = None
    shares_for_1m = None
    if total_shares > 0:
        per_share_underlying = Decimal(reported) / Decimal(total_shares) if reported > 0 else Decimal(0)
        if per_share_underlying > 0:
            shares_for_1m = Decimal(1_000_000) / per_share_underlying
    # If rate is zero or start/principal is zero, skip calculations
    if Decimal(rate_str or "0") == 0 or start == 0 or principal_amt == 0:
        return {
            "vault_address": vault_addr,
            "node": node,
            "vault_share": share_denom,
            "total_shares_amount": str(total_shares),
            "underlying_asset": underlying,
            "principal_amount": str(principal_amt),
            "current_interest_rate": rate_str,
            "period_start": dt_utc_str(start),
            "period_timeout": dt_utc_str(timeout),
            "calc_low": str(reported),
            "calc_high": str(reported),
            "reported": str(reported),
            "in_range": True,
            "range_diff": "0",
            "seconds_low_used": 0,
            "seconds_high_used": 0,
            "per_share_underlying": (str(per_share_underlying) if per_share_underlying not in (None, Decimal(0)) else None),
            "shares_for_1m_uylds": (str(shares_for_1m) if shares_for_1m not in (None, Decimal(0)) else None),
        }
    #calculate tvv at +/- 2 seconds from now
    ts_chain = clamp(now_ts, start, timeout if timeout > 0 else None)
    ts_lo = clamp(ts_chain - 2, start, timeout if timeout > 0 else None)
    ts_hi = clamp(ts_chain + 2, start, timeout if timeout > 0 else None)
    elapsed_at = max(0, ts_chain - start)
    elapsed_lo = max(0, ts_lo - start)
    elapsed_hi = max(0, ts_hi - start)
    calc_at = calc_tvv_continuous(principal_amt, rate_str, elapsed_at)
    calc_lo = calc_tvv_continuous(principal_amt, rate_str, elapsed_lo)
    calc_hi = calc_tvv_continuous(principal_amt, rate_str, elapsed_hi)
    in_range = reported >= calc_lo and reported <= calc_hi
    return {
        "vault_address": vault_addr,
        "node": node,
        "vault_share": share_denom,
        "total_shares_amount": str(total_shares),
        "underlying_asset": underlying,
        "principal_amount": str(principal_amt),
        "current_interest_rate": rate_str,
        "period_start": dt_utc_str(start),
        "period_timeout": dt_utc_str(timeout),
        "calc_low": str(calc_lo), # status block time - 2 seconds
        "calc_high": str(calc_hi), # status block time + 2 seconds
        "calc_at": str(calc_at), # tvv from status query might differ from when vault query happened
        "reported": str(reported), # tvv from endpoint
        "in_range": in_range, # is reported tvv within calculated range
        "range_diff": str(calc_hi - calc_lo),
        "seconds_low_used": elapsed_lo,
        "seconds_high_used": elapsed_hi,
        "per_share_underlying": (str(per_share_underlying) if per_share_underlying not in (None, Decimal(0)) else None),
        "shares_for_1m_uylds": (str(shares_for_1m) if shares_for_1m not in (None, Decimal(0)) else None),
        "now_ts": dt_utc_str(now_ts),
        "seconds_since_start": now_ts - start,
    }

def fetch_one(base: str, target: str) -> dict:
    url = f"{base}/vault/v1/vaults/{quote(target, safe='')}"
    return http_get_json(url)

def fetch_all_addresses(base: str) -> list[str]:
    addrs = []
    next_key = None
    while True:
        params = {}
        if next_key:
            params["pagination.key"] = next_key
        qs = f"?{urlencode(params)}" if params else ""
        url = f"{base}/vault/v1/vaults{qs}"
        data = http_get_json(url)
        for v in data.get("vaults", []) or []:
            addr = v.get("base_account", {}).get("address")
            if addr:
                addrs.append(addr)
        next_key = (data.get("pagination") or {}).get("next_key")
        if not next_key:
            break
    return addrs

def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("-n", dest="network_key", default="", help="pl-testnet | pio-mainnet | pio-testnet | <api-base-url>")
    parser.add_argument("-h", action="store_true", dest="help")
    parser.add_argument("target", nargs="?", default=None)
    args = parser.parse_args()
    if args.help:
        print("""Usage:
  vaults_report.py [-n pl-testnet|pio-mainnet|pio-testnet|<api-base-url>] [vault-address-or-share-denom]

Examples:
  vaults_report.py
  vaults_report.py -n pio-mainnet
  vaults_report.py -n https://rest.custom.net:443
  vaults_report.py -n pl-testnet tp1f2222pmyaw74w33ldrnrxnan73z6ayzhe5pxpl
  vaults_report.py -n pl-testnet nu.uylds.nuva""")
        sys.exit(0)
    base = node_from_network(args.network_key or "")
    now_ts = chain_now_ts(base)
    results = []
    if args.target:
        data = fetch_one(base, args.target)
        results.append(build_report(base, data, now_ts))
    else:
        for addr in fetch_all_addresses(base):
            data = fetch_one(base, addr)
            results.append(build_report(base, data, now_ts))
    print(json.dumps(results, ensure_ascii=False))

if __name__ == "__main__":
    main()
