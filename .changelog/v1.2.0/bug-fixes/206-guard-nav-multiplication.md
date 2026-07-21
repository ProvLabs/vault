* Guard NAV valuation multiplications with `SafeMul` so an oversized net asset value returns an error instead of panicking the block hook [#206](https://github.com/provlabs/vault/issues/206).
