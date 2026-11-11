/**
 * Session parity oracle: load JSONL fixtures through the TypeScript session and
 * print the derived model-visible messages.
 *
 * Usage: pnpm exec tsx go/scripts/session-oracle.ts <file...>
 * Output: one JSON line per file: {"file": "...", "messages": [...]} or
 *         {"file": "...", "error": "..."} when the fixture needs a plugin
 *         projection this script does not supply.
 */
import { readFileSync } from 'node:fs'
import { Session } from '@deepseek-ai/dsh-session'

const files = process.argv.slice(2)
for (const path of files) {
  try {
    const lines = readFileSync(path, 'utf8').trim().split('\n').map(line => JSON.parse(line))
    const header = { ...lines[0], cwd: '/tmp', id: 'session-oracle' }
    const events = lines.slice(1).map((event: any, index: number) => ({ ...event, seq: index, time: 0 }))
    const session = Session.fromRestore(header.id, events, header, 0, 'detached')
    console.log(JSON.stringify({ file: path, messages: session.deriveMessages() }))
  } catch (error) {
    console.log(JSON.stringify({ file: path, error: String(error instanceof Error ? error.message : error) }))
  }
}
