#!/usr/bin/env python3
"""Compare derived session messages from the TypeScript oracle and the Go dump.

Both inputs are JSONL files with one object per fixture:
  {"file": "...", "messages": [...]} or {"file": "...", "error": "..."}

Fixtures the TypeScript oracle cannot load (plugin-owned message projections or
templated tool-addition references) are skipped: they cannot serve as an oracle.

Usage:
  python3 session_parity.py <oracle.jsonl> <go.jsonl>
"""
import json
import sys


def load(path):
    rows = {}
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            entry = json.loads(line)
            rows[entry["file"]] = entry
    return rows


def canonical(messages):
    return json.dumps(messages, sort_keys=True, separators=(",", ":"))


def main(argv):
    if len(argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    oracle = load(argv[1])
    go = load(argv[2])

    compared = skipped = failures = 0
    for path in sorted(oracle):
        left = oracle[path]
        right = go.get(path)
        if right is None:
            print(f"{path}: missing from the Go dump")
            failures += 1
            continue
        if "error" in left:
            skipped += 1
            continue
        if "error" in right:
            print(f"{path}: Go failed: {right['error']}")
            failures += 1
            continue
        compared += 1
        if canonical(left["messages"]) != canonical(right["messages"]):
            print(f"{path}: messages differ")
            print(f"  typescript: {canonical(left['messages'])[:400]}")
            print(f"  go:         {canonical(right['messages'])[:400]}")
            failures += 1
            if failures >= 10:
                print("too many differences, stopping")
                break

    print(f"compared={compared} skipped={skipped} failures={failures}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
