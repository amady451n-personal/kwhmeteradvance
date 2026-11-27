import test from 'node:test'
import assert from 'node:assert/strict'
import { nearestIndex } from './chart.ts'

test('nearestIndex picks the closest point, not only an exact hit', () => {
  const xs = [0.05, 0.3, 0.62, 0.95]
  assert.equal(nearestIndex(0.02, xs), 0)
  assert.equal(nearestIndex(0.2, xs), 1)
  assert.equal(nearestIndex(0.5, xs), 2)
  assert.equal(nearestIndex(0.99, xs), 3)
})

test('nearestIndex clamps outside the plot and handles empty input', () => {
  assert.equal(nearestIndex(-1, [0.2, 0.8]), 0)
  assert.equal(nearestIndex(2, [0.2, 0.8]), 1)
  assert.equal(nearestIndex(0.5, []), null)
})
