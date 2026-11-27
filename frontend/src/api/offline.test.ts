import test from 'node:test'
import assert from 'node:assert/strict'

class Memory {
  m = new Map<string, string>()
  getItem(k: string) { return this.m.get(k) ?? null }
  setItem(k: string, v: string) { this.m.set(k, v) }
  removeItem(k: string) { this.m.delete(k) }
}
Object.defineProperty(globalThis, 'localStorage', { value: new Memory(), configurable: true })

const o = await import('./offline.ts')

test('PIN trace verifies, never stores PIN', async () => {
  assert.equal(o.hasPinTrace(), false)
  await o.rememberPin('123456')
  assert.equal(o.hasPinTrace(), true)
  assert.equal(await o.verifyPin('123456'), true)
  assert.equal(await o.verifyPin('000000'), false)
  assert.ok(!JSON.stringify(localStorage).includes('123456'))
})

test('data key exists without PIN and survives PIN change', async () => {
  const first = await o.ensureDataKey()
  const raw = localStorage.getItem('meteran.key.v1')
  await o.rememberPin('654321')
  const second = await o.ensureDataKey()
  assert.equal(localStorage.getItem('meteran.key.v1'), raw)
  assert.equal(await o.verifyPin('654321'), true)
  assert.ok(first && second)
})

test('seal/open text and snapshot round trip', async () => {
  const sealed = await o.sealText('rahasia-token')
  assert.ok(!sealed.includes('rahasia-token'))
  assert.equal(await o.openText(sealed), 'rahasia-token')
  await o.saveSnapshot({ readings: [1, 2] })
  assert.deepEqual(await o.loadSnapshot(), { readings: [1, 2] })
})

test('broken snapshot returns null without deleting trace', async () => {
  localStorage.setItem('meteran.snapshot.v1', '{rusak')
  assert.equal(await o.loadSnapshot(), null)
  assert.equal(o.hasPinTrace(), true)
})
