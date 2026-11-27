import test from 'node:test'
import assert from 'node:assert/strict'
import { shouldRefreshAfterDrain } from './queue-refresh.ts'

test('refresh only when a drained queue became empty', () => {
  assert.equal(shouldRefreshAfterDrain(0, 0), false)
  assert.equal(shouldRefreshAfterDrain(2, 0), true)
  assert.equal(shouldRefreshAfterDrain(2, 1), false)
  assert.equal(shouldRefreshAfterDrain(0, 2), false)
})
