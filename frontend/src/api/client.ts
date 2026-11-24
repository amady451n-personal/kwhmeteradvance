import type { BlobResult } from './http'
import { request, requestBlob, requestForm } from './http'
import type {
  Homepage,
  ImportCommitResult,
  ImportPreview,
  PageMeta,
  Page,
  PushSubscription,
  PushSubscriptionInput,
  Reading,
  ReadingInput,
  SessionState,
  Settings,
  SettingsPatch,
  Summary,
  Token,
  TokenCancelResult,
  TokenCreate,
  TokenMarkUsed,
  TokenMarkUsedResult,
  TokenPatch,
  TokenUpsert,
  UsageBucket,
} from './types'

export interface ReadingQuery {
  from?: string
  to?: string
  limit?: number
  cursor?: string
}

export interface TokenQuery {
  status?: Token['status']
  limit?: number
  cursor?: string
}

export interface UsageQuery {
  from: string
  to: string
  group: 'day' | 'week' | 'month'
}

export interface UsagePage {
  data: UsageBucket[]
  meta: PageMeta & {
    totalKwh?: number
    totalCost?: number | null
    group?: 'day' | 'week' | 'month'
  }
}

export interface ApiClient {
  readonly mode: 'server' | 'preview'

  getSession(): Promise<SessionState>
  login(pin: string): Promise<void>
  logout(): Promise<void>
  setPin(currentPin: string | null, newPin: string): Promise<void>

  listReadings(query?: ReadingQuery): Promise<Page<Reading>>
  createReading(input: ReadingInput): Promise<Reading>
  upsertReading(id: string, input: ReadingInput): Promise<Reading>
  deleteReading(id: string): Promise<void>

  listTokens(query?: TokenQuery): Promise<Page<Token>>
  createToken(input: TokenCreate): Promise<Token>
  upsertToken(id: string, input: TokenUpsert): Promise<Token>
  patchToken(id: string, patch: TokenPatch): Promise<Token>
  deleteToken(id: string): Promise<void>
  markTokenUsed(id: string, input: TokenMarkUsed): Promise<TokenMarkUsedResult>
  cancelToken(id: string): Promise<TokenCancelResult>
  revealTokenCode(id: string): Promise<string>

  getSettings(): Promise<Settings>
  patchSettings(patch: SettingsPatch): Promise<Settings>

  getSummary(windowDays?: number): Promise<Summary>
  getUsage(query: UsageQuery): Promise<UsagePage>
  getHomepage(): Promise<Homepage>

  importPreview(file: File): Promise<ImportPreview>
  importCommit(previewId: string, confirm: boolean): Promise<ImportCommitResult>

  getReport(from?: string, to?: string): Promise<BlobResult>

  getPushKey(): Promise<string>
  subscribePush(input: PushSubscriptionInput): Promise<PushSubscription>
  unsubscribePush(id: string): Promise<void>
}

interface Envelope<T> {
  data: T
}

export function createHttpClient(): ApiClient {
  async function one<T>(path: string, init?: Parameters<typeof request>[1]): Promise<T> {
    const envelope = await request<Envelope<T>>(path, init)
    return envelope.data
  }

  return {
    mode: 'server',

    getSession: () => one<SessionState>('/api/session'),
    login: (pin) => request('/api/session', { method: 'POST', body: { pin } }),
    logout: () => request('/api/session', { method: 'DELETE' }),
    setPin: (currentPin, newPin) =>
      request('/api/session/pin', { method: 'PUT', body: { currentPin, newPin } }),

    listReadings: (query = {}) =>
      request<Page<Reading>>('/api/readings', { query: { ...query } }),
    createReading: (input) => one<Reading>('/api/readings', { method: 'POST', body: input }),
    upsertReading: (id, input) =>
      one<Reading>(`/api/readings/${encodeURIComponent(id)}`, { method: 'PUT', body: input }),
    deleteReading: (id) =>
      request(`/api/readings/${encodeURIComponent(id)}`, { method: 'DELETE' }),

    listTokens: (query = {}) => request<Page<Token>>('/api/tokens', { query: { ...query } }),
    createToken: (input) => one<Token>('/api/tokens', { method: 'POST', body: input }),
    upsertToken: (id, input) =>
      one<Token>(`/api/tokens/${encodeURIComponent(id)}`, { method: 'PUT', body: input }),
    patchToken: (id, patch) =>
      one<Token>(`/api/tokens/${encodeURIComponent(id)}`, { method: 'PATCH', body: patch }),
    deleteToken: (id) =>
      request(`/api/tokens/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    markTokenUsed: (id, input) =>
      one<TokenMarkUsedResult>(`/api/tokens/${encodeURIComponent(id)}/mark-used`, {
        method: 'POST',
        body: input,
      }),
    cancelToken: (id) =>
      one<TokenCancelResult>(`/api/tokens/${encodeURIComponent(id)}/cancel`, { method: 'POST' }),
    revealTokenCode: (id) =>
      one<{ code: string }>(`/api/tokens/${encodeURIComponent(id)}/reveal`, {
        method: 'POST',
      }).then((payload) => payload.code),

    getSettings: () => one<Settings>('/api/settings'),
    patchSettings: (patch) => one<Settings>('/api/settings', { method: 'PATCH', body: patch }),

    getSummary: (windowDays = 7) =>
      one<Summary>('/api/summary', { query: { window: windowDays } }),
    getUsage: (query) => request<UsagePage>('/api/usage', { query: { ...query } }),
    getHomepage: () => request<Homepage>('/api/homepage'),

    importPreview: (file) => {
      const form = new FormData()
      form.set('file', file)
      return requestForm<Envelope<ImportPreview>>('/api/import/preview', form).then(
        (envelope) => envelope.data,
      )
    },
    importCommit: (previewId, confirm) =>
      one<ImportCommitResult>('/api/import/commit', {
        method: 'POST',
        body: { previewId, confirm },
      }),

    getReport: (from, to) =>
      requestBlob('/api/reports/summary.pdf', { query: { from, to } }),

    getPushKey: () => one<{ publicKey: string }>('/api/push/public-key').then((data) => data.publicKey),
    subscribePush: (input) => one<PushSubscription>('/api/push/subscriptions', {
      method: 'POST',
      body: input,
    }),
    unsubscribePush: (id) =>
      request(`/api/push/subscriptions/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  }
}