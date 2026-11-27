import type { ReadingInput, TokenCreate } from './types'
import { openText, sealText } from './offline.ts'

const V1_KEY = 'meteran.queue.v1'
const KEY = 'meteran.queue.v2'
type Delivery = 'pending' | 'failed' | 'needs_confirm'

export interface QueuedReading {
  kind: 'reading'
  id: string
  input: ReadingInput
  queuedAt: number
  delivery?: Delivery
  error?: string
  confirm?: string[]
}

export interface QueuedToken {
  kind: 'token'
  id: string
  input: TokenCreate
  queuedAt: number
  delivery: Delivery
  error?: string
  confirm?: string[]
}

export type Operation = QueuedReading | QueuedToken

export type DrainOutcome =
  | { ok: true }
  | { ok: false; retryable: boolean; message: string; needsConfirm?: string[] }

interface Stored {
  kind: Operation['kind']
  id: string
  queuedAt: number
  delivery?: Delivery
  error?: string
  confirm?: string[]
  sealed: string
}

function isInactive(item: Operation): boolean {
  return item.delivery === 'failed' || item.delivery === 'needs_confirm'
}

function metadata(item: Operation): Omit<Stored, 'sealed'> {
  return {
    kind: item.kind,
    id: item.id,
    queuedAt: item.queuedAt,
    delivery: item.delivery,
    error: item.error,
    confirm: item.confirm,
  }
}

let chain: Promise<unknown> = Promise.resolve()
function serialize<T>(work: () => Promise<T>): Promise<T> {
  const result = chain.then(work, work)
  chain = result.then(
    () => undefined,
    () => undefined,
  )
  return result
}

async function saveInternal(items: Operation[]): Promise<void> {
  const stored: Stored[] = []
  for (const item of items) {
    stored.push({ ...metadata(item), sealed: await sealText(JSON.stringify(item.input)) })
  }
  try {
    localStorage.setItem(KEY, JSON.stringify(stored))
  } catch {
    // A full or blocked store must not break the in-memory queue.
  }
}

async function loadInternal(): Promise<Operation[]> {
  let raw = localStorage.getItem(KEY)
  if (!raw) {
    const legacy = localStorage.getItem(V1_KEY)
    if (!legacy) return []
    try {
      const old = JSON.parse(legacy) as QueuedReading[]
      const migrated: Stored[] = []
      for (const item of old) {
        if (!item || typeof item.id !== 'string' || !item.input) continue
        migrated.push({ ...metadata(item), sealed: await sealText(JSON.stringify(item.input)) })
      }
      await saveInternal(
        await Promise.all(
          migrated.map(async ({ sealed, ...meta }) => ({
            ...meta,
            input: JSON.parse(await openText(sealed)),
          })),
        ).then((items) => items as Operation[]),
      )
      localStorage.removeItem(V1_KEY)
      raw = localStorage.getItem(KEY)
    } catch {
      localStorage.removeItem(V1_KEY)
      return []
    }
  }

  let stored: Stored[]
  try {
    stored = JSON.parse(raw ?? '[]') as Stored[]
  } catch {
    localStorage.removeItem(KEY)
    return []
  }
  const items: Operation[] = []
  let dropped = false
  for (const entry of stored) {
    try {
      if (!entry || typeof entry.id !== 'string' || !entry.sealed) {
        dropped = true
        continue
      }
      const { sealed, ...meta } = entry
      const input = JSON.parse(await openText(sealed))
      items.push({ ...meta, input } as Operation)
    } catch {
      dropped = true
    }
  }
  if (dropped) await saveInternal(items)
  return items
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

export function enqueueReading(id: string, input: ReadingInput): Promise<number> {
  return serialize(async () => {
    const items = (await loadInternal()).filter((item) => item.id !== id)
    items.push({ kind: 'reading', id, input, queuedAt: Date.now() })
    await saveInternal(items)
    return items.length
  })
}

export function enqueueToken(id: string, input: TokenCreate): Promise<number> {
  return serialize(async () => {
    const items = await loadInternal()
    const previous = items.find((item) => item.kind === 'token' && item.id === id) as
      | QueuedToken
      | undefined
    const next = items.filter((item) => item.id !== id)
    const pending = previous?.delivery === 'pending'
    next.push({
      kind: 'token',
      id,
      input: pending ? input : { ...input, confirm: input.confirm ?? [] },
      queuedAt: previous?.queuedAt ?? Date.now(),
      delivery: 'pending',
      error: pending ? previous?.error : undefined,
      confirm: pending ? previous?.confirm : undefined,
    })
    await saveInternal(next)
    return next.length
  })
}

export function getPending(): Promise<QueuedToken[]> {
  return serialize(async () =>
    (await loadInternal()).filter((item): item is QueuedToken => item.kind === 'token'),
  )
}

export function removePending(id: string): Promise<number> {
  return serialize(async () => {
    const items = (await loadInternal()).filter((item) => !(item.kind === 'token' && item.id === id))
    await saveInternal(items)
    return items.length
  })
}

export function confirmPending(id: string): Promise<number> {
  return serialize(async () => {
    const items = await loadInternal()
    for (const item of items) {
      if (item.kind !== 'token' || item.id !== id || item.delivery !== 'needs_confirm') continue
      item.input = { ...item.input, confirm: (item.confirm ?? []) as TokenCreate['confirm'] }
      item.delivery = 'pending'
      item.error = undefined
      item.confirm = undefined
    }
    await saveInternal(items)
    return items.filter((item) => !isInactive(item)).length
  })
}

/** Reading payloads replay before token payloads; each keeps its insertion order. */
export function drainQueue(
  upsertReading: (id: string, input: ReadingInput) => Promise<DrainOutcome>,
  createToken: (id: string, input: TokenCreate) => Promise<DrainOutcome>,
): Promise<number> {
  return serialize(async () => {
    const items = (await loadInternal()).sort(
      (a, b) => (a.kind === 'reading' ? 0 : 1) - (b.kind === 'reading' ? 0 : 1),
    )
    for (let index = 0; index < items.length; index++) {
      const item = items[index]
      if (isInactive(item)) continue
      const outcome =
        item.kind === 'reading'
          ? await upsertReading(item.id, item.input as ReadingInput)
          : await createToken(item.id, item.input as TokenCreate)
      if (outcome.ok) {
        items.splice(index, 1)
        index--
        await saveInternal(items)
        continue
      }
      if (outcome.retryable) continue
      item.delivery = outcome.needsConfirm?.length ? 'needs_confirm' : 'failed'
      item.confirm = outcome.needsConfirm
      item.error = outcome.message
      await saveInternal(items)
    }
    return items.filter((item) => !isInactive(item)).length
  })
}

export { KEY as QUEUE_KEY }
