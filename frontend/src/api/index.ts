import { createHttpClient } from './client'
import type { ApiClient } from './client'
import { createPreviewClient } from './preview'

export type { ApiClient, ReadingQuery, TokenQuery, UsagePage, UsageQuery } from './client'
export { ApiError } from './http'
export * from './types'

let client: ApiClient | null = null

export function apiClient(): ApiClient {
  if (!client) {
    client =
      import.meta.env.VITE_API_MODE === 'server' ? createHttpClient() : createPreviewClient()
  }
  return client
}

export const apiMode = (): ApiClient['mode'] => apiClient().mode
