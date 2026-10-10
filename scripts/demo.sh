#!/usr/bin/env bash
# Demo: an AI agent talks to a (fake) GitHub MCP server through the engine.
# Run it with `make demo`. Nothing leaves your machine.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -t 1 ]; then
  B=$'\e[1m' DIM=$'\e[2m' G=$'\e[32m' R=$'\e[31m' C=$'\e[36m' Y=$'\e[33m' N=$'\e[0m'
else
  B='' DIM='' G='' R='' C='' Y='' N=''
fi

step() { printf '\n%s━━ %s%s\n\n' "$B$C" "$1" "$N"; }
say()  { printf '%s\n' "$*"; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
evidence="$work/evidence.jsonl"

make --no-print-directory build >/dev/null
go build -o bin/mcp-demo-server ./test/mcp-demo-server

# session <subject> <requests...>: runs one agent session through the proxy
# and prints what the agent receives and what the engine decided.
session() {
  local subject=$1; shift
  printf '%s\n' "$@" | ./bin/engine proxy \
      -policy policies/example.yaml -evidence "$evidence" \
      -subject "$subject" -server github \
      -- ./bin/mcp-demo-server \
      >"$work/out" 2>"$work/err"

  grep 'msg=decision' "$work/err" | while read -r line; do
    resource=$(sed -E 's/.*resource=([^ ]+).*/\1/' <<<"$line")
    reason=$(sed -E 's/.*reason=([^ ]+).*/\1/' <<<"$line")
    if grep -q 'allowed=true' <<<"$line"; then
      say "  ${G}✔ ALLOW${N}  $resource  ${DIM}($reason)${N}"
    else
      say "  ${R}✘ DENY ${N}  $resource  ${DIM}($reason)${N}"
    fi
  done

  say ""
  say "  ${DIM}what the agent received:${N}"
  while read -r line; do summarize "$line"; done <"$work/out" | sort -n | cut -f2-
}

# summarize <json line>: prints "<id><TAB>  ← #<id> <one-line summary>".
summarize() {
  local line=$1 id msg
  id=$(grep -oE '"id":[0-9]+' <<<"$line" | head -1 | cut -d: -f2)
  if grep -q '"error"' <<<"$line"; then
    msg="${R}error $(grep -oE '"code":-?[0-9]+' <<<"$line" | cut -d: -f2): $(grep -oE '"message":"[^"]*"' <<<"$line" | cut -d'"' -f4)${N}"
  elif grep -q '"tools":\[' <<<"$line"; then
    names=$( (grep -oE '"name":"[a-z_]+"' <<<"$line" || true) | cut -d'"' -f4 | paste -sd, - | sed 's/,/, /g')
    msg="tools/list → ${names:-(empty)}"
  elif grep -q '"text":"ran ' <<<"$line"; then
    msg="${G}result: the server ran $(grep -oE '"text":"ran [a-z_]+' <<<"$line" | cut -d' ' -f2)${N}"
  elif grep -q '"serverInfo"' <<<"$line"; then
    msg="initialize → connected to $(grep -oE '"name":"[^"]+"' <<<"$line" | cut -d'"' -f4)"
  else
    msg=$(cut -c1-100 <<<"$line")
  fi
  printf '%s\t  %s←%s #%s %s\n' "${id:-0}" "$DIM" "$N" "${id:-?}" "$msg"
}

init='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
list='{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
call() { printf '{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":{"name":"%s"}}' "$1" "$2"; }

step "1. The policy"
grep -v '^\s*#' policies/example.yaml | sed '/^$/d; s/^/  /'

step "2. agent:billing-bot connects to the GitHub MCP server"
say "  It lists the tools, then calls create_issue and delete_repo."
say "  ${DIM}delete_repo is hidden from its tool list, and blocked when it tries anyway.${N}"
say ""
session agent:billing-bot "$init" "$list" "$(call 3 create_issue)" "$(call 4 delete_repo)"

step "3. agent:intern, an agent the policy does not know"
say "  ${DIM}Deny by default: no rule matches, so nothing is allowed.${N}"
say ""
session agent:intern "$init" "$list" "$(call 3 list_repos)"

step "4. Every decision is in a hash-chained evidence log"
sed 's/^/  /' "$evidence" | cut -c1-160
say ""
./bin/engine verify -evidence "$evidence" | sed "s/^/  ${G}/; s/$/${N}/"

step "5. Someone edits the log to hide the denied delete_repo"
sed -i 's/"allowed":false,"rule_id":"deny-github-delete-repo"/"allowed":true,"rule_id":"deny-github-delete-repo"/' "$evidence"
say "  ${Y}\"allowed\":false  →  \"allowed\":true${N}"
say ""
if ./bin/engine verify -evidence "$evidence" 2>"$work/verr"; then
  say "  ${R}tampering NOT detected${N}"; exit 1
fi
say "  ${R}$(cat "$work/verr")${N}"
say ""
say "  ${DIM}The engine also refuses to append to a broken log:${N}"
session agent:billing-bot "$init" >/dev/null 2>&1 || true
say "  ${R}$(grep -o 'msg="cannot open evidence log".*' "$work/err" | cut -c1-120)${N}"

printf '\n%s━━ Done.%s Wrap any MCP server the same way: see README.md, "Use it with an MCP client".\n\n' "$B$C" "$N"
