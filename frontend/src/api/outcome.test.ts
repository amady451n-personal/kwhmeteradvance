import test from 'node:test'
import assert from 'node:assert/strict'
import { ApiError } from './http.ts'

const { toDrainOutcome } = await import('./outcome.ts')

function retryableOf(outcome: import('./queue.ts').DrainOutcome): boolean {
  return !outcome.ok && outcome.retryable
}

test('401 and 429 stay retryable and are not dead-lettered', () => {
  assert.deepEqual(toDrainOutcome(new ApiError(401, 'unauthorized', 'Perlu masuk')), {
    ok: false,
    retryable: true,
    message: 'Perlu masuk',
  })
  assert.deepEqual(toDrainOutcome(new ApiError(429, 'rate_limited', 'Tunggu')), {
    ok: false,
    retryable: true,
    message: 'Tunggu',
  })
})

test('confirmation_required carries codes for user confirmation', () => {
  const error = new ApiError(409, 'confirmation_required', 'Perlu konfirmasi.', [
    { code: 'denomination_out_of_tolerance', message: 'Di luar toleransi.' },
  ])
  assert.deepEqual(toDrainOutcome(error), {
    ok: false,
    retryable: false,
    message: 'Perlu konfirmasi.',
    needsConfirm: ['denomination_out_of_tolerance'],
  })
})

test('network and 5xx retry; hard validation does not', () => {
  assert.equal(retryableOf(toDrainOutcome(new ApiError(0, 'network_error', 'Offline'))), true)
  assert.equal(retryableOf(toDrainOutcome(new ApiError(500, 'internal_error', 'Gagal'))), true)
  assert.equal(retryableOf(toDrainOutcome(new ApiError(422, 'validation_error', 'Salah'))), false)
  assert.equal('needsConfirm' in toDrainOutcome(new ApiError(422, 'validation_error', 'Salah')), false)
})
