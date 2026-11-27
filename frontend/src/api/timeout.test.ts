import test from 'node:test'
import assert from 'node:assert/strict'

const { withTimeout } = await import('./timeout.ts')

test('withTimeout releases a hung promise', async () => {
  const never = new Promise<string>(() => {})
  const result = await withTimeout(never, 20, () => 'fallback')
  assert.equal(result, 'fallback')
})

test('withTimeout keeps a fast result', async () => {
  const result = await withTimeout(Promise.resolve('fast'), 100, () => 'fallback')
  assert.equal(result, 'fast')
})
