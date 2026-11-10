#!/usr/bin/env python3
"""Compare a TypeScript `glh --dump-config` dump with the Go one.

YAML formatting differs (quoting, folding, key order), so both dumps are parsed
structurally. For every row we compare its fields and the layer chain printed in
the preceding `# == ...` header.

Usage:
  python3 parity.py <typescript-dump> <go-dump>
"""
import sys

import yaml

ROOT = object()


def _js(loader, tag_suffix, node):
    if isinstance(node, yaml.ScalarNode):
        return node.value
    if isinstance(node, yaml.SequenceNode):
        return loader.construct_sequence(node)
    return loader.construct_mapping(node)


class Loader(yaml.SafeLoader):
    pass


for tag in ("!js", "!!js", "tag:yaml.org,2002:js"):
    Loader.add_multi_constructor(tag, _js)


def sections(path):
    with open(path, encoding="utf-8") as handle:
        raw = handle.read().splitlines()

    header = None
    buffer = []
    found = []

    def flush():
        if header is None or not buffer:
            return
        for row in yaml.load("\n".join(buffer), Loader=Loader) or []:
            found.append((tuple(part.strip() for part in header.split(", patched by ")), row))

    for line in raw:
        if line.startswith("$ "):
            continue
        if line.startswith("# == "):
            flush()
            buffer = []
            header = line[len("# == "):]
            continue
        buffer.append(line)
    flush()
    return found


def normalise(row):
    out = {}
    for key in ("id", "name", "inject", "config", "disabled"):
        if key in row:
            out[key] = row[key]
    return out


def main(argv):
    if len(argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    ts = sections(argv[1])
    go = sections(argv[2])

    failures = 0
    if len(ts) != len(go):
        print(f"row count: typescript={len(ts)} go={len(go)}")
        failures += 1

    for index, (ts_row, go_row) in enumerate(zip(ts, go)):
        ts_chain, ts_body = ts_row
        go_chain, go_body = go_row
        if ts_chain != go_chain:
            print(f"row {index}: layer chain typescript={ts_chain} go={go_chain}")
            failures += 1
        left, right = normalise(ts_body), normalise(go_body)
        if left != right:
            print(f"row {index} ({left.get('id')}):")
            for key in sorted(set(left) | set(right)):
                if left.get(key) != right.get(key):
                    print(f"  {key}: typescript={left.get(key)!r} go={right.get(key)!r}")
            failures += 1
        if failures > 20:
            print("too many differences, stopping")
            break

    if failures:
        print(f"FAIL: {failures} difference(s)")
        return 1
    print(f"OK: {len(ts)} rows match")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
