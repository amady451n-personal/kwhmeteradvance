import type { ReadingInput, TokenCreate } from './types'
import { openText, sealText } from './offline.ts'

const V1_KEY = 'meteran.queue.v1'
const KEY = 'meteran.queue.v2'

export interface QueuedReading {
  kind: 'reading'
  id: string
  input: ReadingInput
  queuedAt: number
  delivery?: 'pending' | 'failed'
  error?: string
}

export interface QueuedToken {
  kind: 'token'
  id: string
  input: TokenCreate
  queuedAt: number
  delivery: 'pending' | 'failed'
  error?: string
}

export type Operation = QueuedReading | QueuedToken

export type DrainOutcome =
  | { ok: true }
  | { ok: false; retryable: boolean; message: string }

interface Stored {
  kind: Operation['kind']
  id: string
  queuedAt: number
  delivery?: 'pending' | 'failed'
  error?: string
  sealed: string
}

function isFailed(item: Operation): boolean {
  return item.delivery === 'failed'
}

function metadata(item: Operation): Omit<Stored, 'sealed'> {
  return { kind: item.kind, id: item.id, queuedAt: item.queuedAt, delivery: item.delivery, error: item.error }
}

async function load(): Promise<Operation[]> {
  let raw = localStorage.getItem(KEY)
  if (!raw) {
    const legacy = localStorage.getItem(V1_KEY)
    if (!legacy) return []
    const old = JSON.parse(legacy) as QueuedReading[]
    const migrated: Stored[] = []
    for (const item of old) {
      migrated.push({ ...metadata(item), sealed: await sealText(JSON.stringify(item.input)) })
    }
    localStorage.setItem(KEY, JSON.stringify(migrated))
    localStorage.removeItem(V1_KEY)
    raw = localStorage.getItem(KEY)
  }
  const stored = JSON.parse(raw ?? '[]') as Stored[]
  const items: Operation[] = []
  for (const entry of stored) {
    const { sealed, ...meta } = entry
    const input = JSON.parse(await openText(sealed))
    items.push({ ...meta, input } as Operation)
  }
  return items
}

async function save(items: Operation[]): Promise<void> {
  const stored: Stored[] = []
  for (const item of items) {
    stored.push({ ...metadata(item), sealed: await sealText(JSON.stringify(item.input)) })
  }
  localStorage.setItem(KEY, JSON.stringify(stored))
}

/** Plain count: queued payloads stay sealed, only metadata is read. */
export function queueSize(): number {
  const raw = localStorage.getItem(KEY) ?? localStorage.getItem(V1_KEY)
  if (!raw) return 0
  try {
    return (JSON.parse(raw) as unknown[]).length
  } catch {
    return 0
  }
}

export async function enqueueReading(id: string, input: ReadingInput): Promise<number> {
  const items = (await load()).filter((item) => !(item.kind === 'reading' && item.id === id))
  items.push({ kind: 'reading', id, input, queuedAt: Date.now() })
  await save(items)
  return items.length
}

export async function enqueueToken(id: string, input: TokenCreate): Promise<number> {
  const items = await load()
  const existing = items.find((item) => item.kind === 'token' && item.id === id) as
    | QueuedToken
    | undefined
  const next = items.filter((item) => item.id !== id)
  next.push({
    kind: 'token',
    id,
    input,
    queuedAt: existing?.queuedAt ?? Date.now(),
    delivery: existing?.delivery ?? 'pending',
    error: existing?.error,
  })
  await save(next)
  return next.length
}

export async function getPending(): Promise<QueuedToken[]> {
  return (await load()).filter((item): item is QueuedToken => item.kind === 'token')
}

export async function removePending(id: string): Promise<number> {
  const items = (await load()).filter((item) => item.id !== id)
  await save(items)
  return items.length
}

/**
 * Replay in order. A retryable failure stops the drain; a rejected entry is
 * marked failed and the next entry still gets its turn.
 */
export async function drainQueue(
  upsertReading: (id: string, input: ReadingInput) => Promise<DrainOutcome>,
  createToken: (id: string, input: TokenCreate) => Promise<DrainOutcome>,
): Promise<number> {
  const items = (await load()).sort(
    (a, b) => (a.kind === 'reading' ? 0 : 1) - (b.kind === 'reading' ? 0 : 1),
  )
  for (let index = 0; index < items.length; index++) {
    const item = items[index]
    if (isFailed(item)) continue
    const outcome =
      item.kind === 'reading'
        ? await upsertReading(item.id, item.input as ReadingInput)
        : await createToken(item.id, item.input as TokenCreate)
    if (outcome.ok) {
      items.splice(index, 1)
      index--
      await save(items)
      continue
    }
    if (outcome.retryable) continue
    item.delivery = 'failed'
    item.error = outcome.message
    await save(items)
  }
  return items.filter((item) => !isFailed(item)).length
}

export { KEY as QUEUE_KEY }
