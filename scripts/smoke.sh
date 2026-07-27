#!/usr/bin/env zsh
# Live smoke test against the real GW2 API.
#
# Public checks always run. Authenticated checks run only when a credential
# profile is already stored (`gw2 auth set ...`); the key itself is read by
# the CLI from its encrypted store and never appears here, in env vars, or
# in output. Network required; not part of `go test ./...`.
set -euo pipefail

cd "$(dirname "$0")/.."
go build -o gw2 ./cmd/gw2

pass=0
fail=0

check() {
  local name="$1" expect="$2"
  shift 2
  local out
  if out="$("$@" 2>&1)" && [[ "$out" == *"$expect"* ]]; then
    echo "PASS  $name"
    pass=$((pass + 1))
  else
    echo "FAIL  $name"
    echo "      cmd: $*"
    echo "      expected substring: $expect"
    echo "      got: ${out:0:200}"
    fail=$((fail + 1))
  fi
}

echo "== public =="
check "build id"            "build "        ./gw2 build
check "data colors by id"   "Sky"           ./gw2 data colors --ids 10
check "data items by id"    "24"            ./gw2 data items --ids 24
check "commerce prices"     "buy "          ./gw2 commerce prices 19684
check "commerce exchange"   "coins_per_gem" ./gw2 commerce exchange gems --quantity 100
check "wvw ranks"           "Invader"       ./gw2 wvw ranks --ids 1
check "pvp ranks id list"   " "             ./gw2 pvp ranks
check "achievements cats"   " "             ./gw2 achievements categories

if [[ -n "$(./gw2 auth list 2>/dev/null)" ]]; then
  echo "== authed (stored profile) =="
  check "token info"          "permissions:"  ./gw2 token info
  check "account info"        "name"          ./gw2 account
  check "account wallet"      "1"             ./gw2 account wallet
  check "character list"      " "             ./gw2 character
  check "transactions walk"   ""              ./gw2 commerce transactions history sells
else
  echo "== authed checks skipped (no profile stored; run: gw2 auth set <name> --key <key>) =="
fi

echo
echo "smoke: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
