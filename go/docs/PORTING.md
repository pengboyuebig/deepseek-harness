# Go port plan

The Go runtime replaces the harness **host**: profile composition, session log,
agent loop, model adapters, tools, persistence, and the HTTP surface. The React
client, the documentation site, and the TypeScript packages stay in place as the
reference implementation and parity oracle.

## Scope decisions

1. **Web UI stays TypeScript.** `apps/web` builds to `apps/web/dist`, which the
   Go server embeds or serves from disk. Rewriting React in Go buys nothing.
2. **TypeScript plugins do not load.** Cordis is replaced by a Go context with
   services, typed events, waterfalls, and reversible effects. Third-party
   extension happens through MCP or an out-of-process JSON-RPC plugin.
3. **Manifest and wire identifiers stay upstream.** `@deepseek-ai/dsh-*`,
   `"dsh": {...}` package fields, `engines.dsh`, `dshHome`, and the `deepseek`
   provider ids are data contracts, not branding.
4. **`typert` has no direct port.** Model-facing JSON Schemas are generated from
   Go struct tags and reflection instead of a TypeScript type graph.

## Milestones

### G0 — skeleton (done)

- `go.mod`, package layout mirroring the TypeScript packages.
- `plugin.Context`: Provide/Get, On/Emit, OnMiddleware/Waterfall, Effect/Dispose.
- `boot`: patch files, insert/patch application, layered dump with `!!js`
  preserved.
- `session.Log`: append-only events with sequence numbers and `DeriveMessages`.
- `llm.DeepSeek`: OpenAI-compatible SSE streaming with `GLKJ_*` defaults.
- `agent.Agent`: one turn, pre-step waterfall, streaming chunks, durable events.
- `tools.Registry`, `api.Server`, `web.Handler`, `config.Load`, `cmd/glh`.

Acceptance: `go build ./... && go vet ./... && go test ./...`; `glh version`;
`glh dump-config` on a profile.

### G1 — composition parity (done)

- Resolve bundles from the profile manifest (`dsh.profile.bundles`) instead of a
  hard-coded base patch.
- Apply profile → home → `--patch` overlays in the documented order and reject
  patches targeting unknown rows.
- `dump-config` output diffed against the TypeScript binary for the `base`,
  `web`, and `headless` profiles, plus profile-patch and `--patch` overlays.

Acceptance: identical row order, ids, fields, `!!js` values, and provenance
headers; unknown-id patches warn and are skipped with the same exit code as the
TypeScript loader. Long scalar wrapping differs between the YAML emitters, so
`go/scripts/parity.py` compares the parsed tree rather than raw bytes.

### G2 — session parity

- JSONL persistence compatible with `session.v4.jsonl` fixtures.
- `deriveMessages` over every event kind that contributes model history.
- Fork, resume, and turn numbering from a loaded log.
- Replay the repository's `snapshots/session/*` corpus and compare the derived
  message stream.

Acceptance: 100% of snapshot fixtures replay with identical message sequences.

### G3 — agent loop parity

- Turn/step state machine with `agent/pre-step`, `agent/request`, `llm/stream`.
- Tool pipeline `tools/pre-execute → execute → post-execute` with parallel calls
  and cancellation.
- Compaction hooks and turn-stopping.

Acceptance: event-sequence equality against `snapshots/session` expectations for
text turns, tool turns, retries, and cancellation.

### G4 — tools and persistence

- Port `bash`, `read`, `write`, `edit`, `glob`, `grep`, `todo`, `task`,
  `ask_user`, `web`, `skill`, `mcp`, `plan`, `goal`.
- `fs`, `shell`, `subprocess`, `sandbox`, `terminal`, `lsp`, `jobs`, `schedule`
  seams with local providers.
- SQLite session query and spill handling.

Acceptance: headless tasks run end to end; session files are readable by both
runtimes.

### G5 — HTTP surface

- `/api` routes and the event stream consumed by the existing client.
- Static client serving with the same token gate.

Acceptance: the React client works against the Go server with no client changes.

### G6 — plugins and release

- `plugin.Register` interface plus manifest-driven composition.
- One example plugin ported from `examples/`.
- Single-binary release builds for linux/darwin/windows.

Acceptance: a third-party Go plugin compiles, mounts, and unloads cleanly.

## Parity harness

`go test ./...` covers unit behaviour. Cross-implementation parity uses:

- `snapshots/**` golden files as the oracle for session replay and prompt
  assembly;
- `pnpm exec vitest run` in the TypeScript tree for the same fixtures;
- a diff job that runs `glh dump-config` from both binaries and compares output.

## Risks

| Risk | Mitigation |
| --- | --- |
| Cordis effects/disposal semantics drift | the `plugin` package tests encode LIFO disposal and waterfall contracts |
| `!!js` configuration cannot be evaluated in Go | keep values opaque, document the limitation, and read the evaluated values from the TypeScript dump when needed |
| Tool schemas differ from `typert` output | generate from struct tags and diff against `snapshots/*/tool-schemas.expected.json` |
| Scope creep into the web client | the client stays TypeScript; Go only serves its build |
