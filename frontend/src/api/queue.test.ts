import test from 'node:test'
import assert from 'node:assert/strict'

class Memory {
  m = new Map<string, string>()
  getItem(k: string) { return this.m.get(k) ?? null }
  setItem(k: string, v: string) { this.m.set(k, v) }
  removeItem(k: string) { this.m.delete(k) }
}
Object.defineProperty(globalThis, 'localStorage', { value: new Memory(), configurable: true })

const q = await import('./queue.ts')

function reset() {
  localStorage.removeItem('meteran.queue.v1')
  localStorage.removeItem('meteran.queue.v2')
}

reset()

test('migrates v1 readings into v2 without loss', async () => {
  localStorage.setItem('meteran.queue.v1', JSON.stringify([
    { kind: 'reading', id: 'r1', input: { id: 'r1', reading: 90 }, queuedAt: 1 },
  ]))
  assert.equal(q.queueSize(), 1)
  await q.getPending()
  assert.ok(localStorage.getItem('meteran.queue.v2'))
  const seen: string[] = []
  await q.drainQueue(async (id) => { seen.push(id); return { ok: true as const } }, async () => ({ ok: true as const }))
  assert.deepEqual(seen, ['r1'])
})

test('queued token code is sealed', async () => {
  reset()
  await q.enqueueToken('t1', { code: 'RAHASIA123', kwh: 100, cost: 150000 } as never)
  const raw = localStorage.getItem('meteran.queue.v2')!
  assert.ok(!raw.includes('RAHASIA123'))
  assert.equal((await q.getPending())[0].input.code, 'RAHASIA123')
})

test('drain skips network failure, dead-letters 4xx, continues next', async () => {
  reset()
  await q.enqueueToken('d1', { code: 'A', kwh: 1, cost: 1 } as never)
  await q.enqueueToken('d2', { code: 'B', kwh: 2, cost: 2 } as never)
  const seen: string[] = []
  const remaining = await q.drainQueue(
    async () => ({ ok: true as const }),
    async (id) => {
      seen.push(id)
      if (id === 'd1') return { ok: false as const, retryable: true, message: 'offline' }
      return { ok: false as const, retryable: false, message: 'data tidak valid' }
    },
  )
  assert.deepEqual(seen, ['d1', 'd2'])
  assert.equal(remaining, 1)
  const [left] = await q.getPending()
  assert.equal(left.delivery, 'pending')
  assert.equal(left.id, 'd1')
  await q.drainQueue(async () => ({ ok: true as const }), async (id) =>
    id === 'd1' ? { ok: true as const } : ({ ok: true as const }),
  )
})

test('4xx entry is failed, kept, and never replayed', async () => {
  reset()
  await q.enqueueToken('bad', { code: 'X', kwh: 1, cost: 1 } as never)
  await q.drainQueue(
    async () => ({ ok: true as const }),
    async () => ({ ok: false as const, retryable: false, message: 'duplikat' }),
  )
  const [entry] = await q.getPending()
  assert.equal(entry.delivery, 'failed')
  assert.equal(entry.error, 'duplikat')
  const seen: string[] = []
  await q.drainQueue(
    async () => ({ ok: true as const }),
    async (id) => { seen.push(id); return { ok: true as const } },
  )
  assert.deepEqual(seen, [])
  assert.equal(q.queueSize(), 1)
  assert.equal(await q.removePending('bad'), 0)
})

test('drain replays readings before tokens', async () => {
  reset()
  await q.enqueueToken('tok', { code: 'Z', kwh: 1, cost: 1 } as never)
  await q.enqueueReading('r', { id: 'r', reading: 10, timestamp: '2026-11-27T08:00:00+07:00' } as never)
  const seen: string[] = []
  await q.drainQueue(
    async (id) => { seen.push(`reading:${id}`); return { ok: true as const } },
    async (id) => { seen.push(`token:${id}`); return { ok: true as const } },
  )
  assert.deepEqual(seen, ['reading:r', 'token:tok'])
})

test('confirmation_required keeps entry pending for user confirmation', async () => {
  reset()
  await q.enqueueToken('need', { code: 'Y', kwh: 1, cost: 1 } as never)
  await q.drainQueue(
    async () => ({ ok: true as const }),
    async () => ({ ok: false as const, retryable: false, message: 'Perlu konfirmasi.', needsConfirm: ['denomination_out_of_tolerance'] }),
  )
  let [entry] = await q.getPending()
  assert.equal(entry.delivery, 'needs_confirm')
  assert.deepEqual(entry.confirm, ['denomination_out_of_tolerance'])

  const seen = [] as string[]
  await q.drainQueue(
    async () => ({ ok: true as const }),
    async (id) => { seen.push(id); return { ok: true as const } },
  )
  assert.deepEqual(seen, [])

  await q.confirmPending('need')
  let confirmed = 0
  await q.drainQueue(
    async () => ({ ok: true as const }),
    async () => { confirmed++; return { ok: true as const } },
  )
  assert.equal(confirmed, 1)
  assert.equal(q.queueSize(), 0)
})

test('corrupt queue entry is skipped without poisoning the rest', async () => {
  reset()
  await q.enqueueToken('good', { code: 'G', kwh: 1, cost: 1 } as never)
  const raw = JSON.parse(localStorage.getItem('meteran.queue.v2')!)
  raw.splice(0, 0, { kind: 'token', id: 'bad', queuedAt: 0, sealed: '!!!' })
  localStorage.setItem('meteran.queue.v2', JSON.stringify(raw))
  assert.equal((await q.getPending()).some((e) => e.id === 'bad'), false)
  assert.equal((await q.getPending()).some((e) => e.id === 'good'), true)
})

test('concurrent enqueues both survive', async () => {
  reset()
  await Promise.all([
    q.enqueueToken('a', { code: 'A', kwh: 1, cost: 1 } as never),
    q.enqueueToken('b', { code: 'B', kwh: 2, cost: 2 } as never),
  ])
  assert.deepEqual((await q.getPending()).map((e) => e.id).sort(), ['a', 'b'])
})

test('pending readings are visible and removable without touching tokens', async () => {
  reset()
  await q.enqueueReading('r1', { id: 'r1', reading: 90, timestamp: '2026-11-27T08:00:00+07:00' } as never)
  await q.enqueueToken('t1', { code: 'A', kwh: 1, cost: 1 } as never)
  assert.equal((await q.getPendingReadings()).length, 1)
  assert.equal(await q.removePending('r1', 'reading'), 1)
  assert.equal((await q.getPending()).length, 1)
})
