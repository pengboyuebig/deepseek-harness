# GLKJ Harness — Go port

This directory holds the Go rewrite of the harness runtime. The TypeScript tree
at the repository root remains the reference implementation and the source of
the web UI; parity is verified against its test fixtures.

## Status

| Milestone | Scope | State |
| --- | --- | --- |
| G0 | module, layout, plugin context, profile composition, session log, LLM SSE client, API server, CLI | done |
| G1 | cordis parity: profile/bundle/patch resolution, `dump-config` byte parity | in progress |
| G2 | session log: JSONL persistence, `deriveMessages`, fork/resume, snapshot replay | pending |
| G3 | agent loop: turn/step state machine, tool pipeline, streaming handlers | pending |
| G4 | tool set + fs/shell/subprocess/sandbox, sqlite session query, headless | pending |
| G5 | `/api` gateway + SSE, embedded React client | pending |
| G6 | Go plugin interface, packaging and release | pending |

## Layout

| Go package | TypeScript counterpart |
| --- | --- |
| `internal/plugin` | `vendor/cordis` (Context, events, effects) |
| `internal/boot` | `packages/boot/app-boot` |
| `internal/core/session` | `packages/core/session` |
| `internal/core/agent` | `packages/core/agent`, `packages/core/agent-loop` |
| `internal/core/tools` | `packages/core/tools`, `packages/core/scope` |
| `internal/llm` | `packages/llm/llm`, `packages/llm/llm-deepseek` |
| `internal/api` | `packages/api` |
| `internal/web` | `apps/web` build output only (React stays TypeScript) |
| `internal/config` | `packages/util/home-paths`, `packages/settings` |

## Commands

```sh
go build ./...
go vet ./...
go test ./...
go build -o bin/glh ./cmd/glh
GLH_HOME=~/.glh ./bin/glh --profile web dump-config
./bin/glh --profile web web --port 3080
```

The binary serves `apps/web/dist` when it exists next to the repository, or
`$GLH_WEB_DIST`; otherwise it serves a placeholder page explaining how to build
the client.

## Deliberate divergences

- The `@deepseek-ai/dsh-*` package namespace, `"dsh": {...}` manifest keys, and
  the `deepseek` provider/model ids stay as-is: they are wire and manifest
  identifiers, not brand copy.
- Cordis plugins written in TypeScript cannot load into this runtime. The Go
  plugin interface is defined in G6; cross-language extension goes through MCP.
- `!!js` values in patch files are preserved verbatim by `dump-config` and are
  not evaluated: Go has no equivalent of evaluating configuration JavaScript.
