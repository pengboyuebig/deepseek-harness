#!/bin/sh
# Compare derived session messages between the TypeScript oracle and the Go
# runtime across every *.v4.jsonl fixture under snapshots/.
set -eu
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

files=$(find snapshots -name '*.v4.jsonl' | sort)
oracle=$(mktemp)
dump=$(mktemp)
binary=$(mktemp -u)
trap 'rm -f "$oracle" "$dump" "$binary"' EXIT

pnpm exec tsx go/scripts/session-oracle.ts $files >"$oracle"
(cd go && go build -o "$binary" ./cmd/sessiondump)
"$binary" $files >"$dump"
python3 go/scripts/session_parity.py "$oracle" "$dump"
