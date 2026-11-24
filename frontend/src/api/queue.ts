import type { ReadingInput } from './types'

const KEY = 'meteran.queue.v1'

export interface QueuedReading {
  kind: 'reading'
  id: string
  input: ReadingInput
  queuedAt: number
}

type Operation = QueuedReading

function load(): Operation[] {
  try {
    const raw = localStorage.getItem(KEY)
    return raw ? (JSON.parse(raw) as Operation[]) : []
  } catch {
    return []
  }
}

function save(items: Operation[]) {
  localStorage.setItem(KEY, JSON.stringify(items))
}

export function queueSize(): number {
  return load().length
}

export function enqueueReading(id: string, input: ReadingInput): number {
  const items = load().filter((item) => !(item.kind === 'reading' && item.id === id))
  items.push({ kind: 'reading', id, input, queuedAt: Date.now() })
  save(items)
  return items.length
}

/** Replay in order; server upserts are idempotent per ID. */
export async function drainQueue(
  upsertReading: (id: string, input: ReadingInput) => Promise<unknown>,
): Promise<number> {
  let pending = load()
  while (pending.length > 0) {
    const [next] = pending
    try {
      await upsertReading(next.id, next.input)
    } catch {
      break
    }
    pending = pending.filter((item) => item !== next)
    save(pending)
  }
  return pending.length
}

export { KEY as QUEUE_KEY }