import type { ErrorBody, ErrorCode, ErrorDetail } from './types'

export class ApiError extends Error {
  readonly status: number
  readonly code: ErrorCode | 'network_error' | 'unsupported_in_preview'
  readonly details: ErrorDetail[]
  readonly retryAfter: number | null

  constructor(
    status: number,
    code: ErrorCode | 'network_error' | 'unsupported_in_preview',
    message: string,
    details: ErrorDetail[] = [],
    retryAfter: number | null = null,
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
    this.retryAfter = retryAfter
  }

  get isOffline(): boolean {
    return this.code === 'network_error'
  }

  get needsConfirmation(): boolean {
    return this.code === 'confirmation_required'
  }

  detailCodes(): string[] {
    return this.details.map((detail) => detail.code)
  }
}

export type Query = Record<string, string | number | boolean | null | undefined>

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  query?: Query
  signal?: AbortSignal
}

function withQuery(path: string, query?: Query): string {
  if (!query) return path
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue
    params.set(key, String(value))
  }
  const search = params.toString()
  return search ? `${path}?${search}` : path
}

async function failure(response: Response): Promise<ApiError> {
  const retryAfter = Number(response.headers.get('Retry-After'))
  let payload: { error?: ErrorBody } | null = null
  try {
    payload = (await response.json()) as { error?: ErrorBody }
  } catch {
    payload = null
  }

  const body = payload?.error
  return new ApiError(
    response.status,
    body?.code ?? 'internal_error',
    body?.message ?? `Permintaan gagal (${response.status}).`,
    body?.details ?? [],
    Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : null,
  )
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (options.body !== undefined) headers['Content-Type'] = 'application/json'

  let response: Response
  try {
    response = await fetch(withQuery(path, options.query), {
      method: options.method ?? 'GET',
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      credentials: 'include',
      signal: options.signal,
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new ApiError(0, 'network_error', 'Tidak ada koneksi ke server.')
  }

  if (!response.ok) throw await failure(response)
  if (response.status === 204) return undefined as T
  const text = await response.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export interface BlobResult {
  blob: Blob
  filename: string | null
}

export async function requestBlob(
  path: string,
  options: RequestOptions = {},
): Promise<BlobResult> {
  let response: Response
  try {
    response = await fetch(withQuery(path, options.query), {
      method: options.method ?? 'GET',
      credentials: 'include',
      signal: options.signal,
    })
  } catch {
    throw new ApiError(0, 'network_error', 'Tidak ada koneksi ke server.')
  }

  if (!response.ok) throw await failure(response)

  const disposition = response.headers.get('Content-Disposition') ?? ''
  const match = disposition.match(/filename="?([^";]+)"?/)
  return { blob: await response.blob(), filename: match ? match[1] : null }
}

export async function requestForm<T>(path: string, form: FormData): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      method: 'POST',
      body: form,
      credentials: 'include',
      headers: { Accept: 'application/json' },
    })
  } catch {
    throw new ApiError(0, 'network_error', 'Tidak ada koneksi ke server.')
  }

  if (!response.ok) throw await failure(response)
  return (await response.json()) as T
}