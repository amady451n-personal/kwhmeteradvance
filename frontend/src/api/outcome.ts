import { ApiError } from './http.ts'
import type { DrainOutcome } from './queue.ts'

/** Map a replay error to a queue outcome without duplicating auth policy. */
export function toDrainOutcome(caught: unknown): DrainOutcome {
  if (!(caught instanceof ApiError)) {
    return {
      ok: false,
      retryable: true,
      message: caught instanceof Error ? caught.message : 'Gagal',
    }
  }
  if (caught.status === 401 || caught.status === 429 || caught.isOffline || caught.status >= 500) {
    return { ok: false, retryable: true, message: caught.message }
  }
  if (caught.needsConfirmation) {
    return {
      ok: false,
      retryable: false,
      message: caught.message,
      needsConfirm: caught.detailCodes(),
    }
  }
  return { ok: false, retryable: false, message: caught.message }
}
