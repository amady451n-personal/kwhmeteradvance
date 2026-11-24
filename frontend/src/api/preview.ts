import fixture from '../data/fixture.json'
import {
  DEFAULT_DENOMINATIONS,
  monthStart,
  readingViews,
  summarize,
  usageBuckets,
} from '../domain/energy'
import type { Reading, TokenRecord } from '../domain/types'
import {
  summaryToApi,
  usageBucketFromView,
  wibIso,
} from './mappers'
import { ApiError } from './http'
import type { ApiClient, ReadingQuery, TokenQuery, UsageQuery, UsagePage } from './client'
import type {
  Homepage,
  Settings,
  SettingsPatch,
  Token,
  TokenCreate,
  TokenPatch,
  TokenUpsert,
} from './types'

const KEY = 'meteran.preview.v2'
const META = (): string => wibIso(Date.now())

interface PreviewSettings {
  buffer: number
  avgWindow: 7 | 14 | 30
  targetHour: number
  reminderHour: number
  denominations: number[]
  tariffPerKwh: number | null
  tariffGolongan: string | null
}

interface PreviewState {
  readings: Reading[]
  tokens: Token[]
  settings: PreviewSettings
  codes: Record<string, string>
}

const defaults: PreviewSettings = {
  buffer: 10,
  avgWindow: 7,
  targetHour: 6,
  reminderHour: 8,
  denominations: [...DEFAULT_DENOMINATIONS],
  tariffPerKwh: 1444.7,
  tariffGolongan: 'R1/1300',
}

const legacy = fixture as unknown as {
  readings: Array<{
    id: number
    meter_name: string
    reading: number
    timestamp: string
    notes: string | null
    recharge_amount: number | null
    cost: number | null
  }>
  tokens: TokenRecord[]
}

function seed(): PreviewState {
  return {
    readings: legacy.readings.map((row) => ({
      id: `seed-r${row.id}`,
      meterName: row.meter_name,
      reading: row.reading,
      timestamp: row.timestamp,
      notes: row.notes,
      rechargeAmount: row.recharge_amount,
      cost: row.cost,
      tokenId: null,
      source: 'import',
      locked: false,
      oldId: String(row.id),
      createdAt: row.timestamp,
      updatedAt: row.timestamp,
    })),
    tokens: [],
    settings: { ...defaults, denominations: [...defaults.denominations] },
    codes: {},
  }
}

function load(): PreviewState {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return seed()
    const parsed = JSON.parse(raw) as Partial<PreviewState>
    if (!Array.isArray(parsed.readings) || !Array.isArray(parsed.tokens)) return seed()
    return {
      readings: parsed.readings,
      tokens: parsed.tokens,
      settings: { ...defaults, ...(parsed.settings ?? {}) },
      codes: parsed.codes ?? {},
    }
  } catch {
    return seed()
  }
}

function newId(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function page<T>(items: T[], limit = 200): { data: T[]; meta: { nextCursor: null; hasMore: boolean; total: number } } {
  const data = items.slice(0, limit)
  return { data, meta: { nextCursor: null, hasMore: items.length > data.length, total: items.length } }
}

function unsupported(feature: string): never {
  throw new ApiError(
    501,
    'unsupported_in_preview',
    `${feature} hanya tersedia saat backend terhubung.`,
  )
}

export function createPreviewClient(): ApiClient {
  const state = load()

  function save() {
    localStorage.setItem(KEY, JSON.stringify(state))
  }

  function findReading(id: string): Reading {
    const found = state.readings.find((row) => row.id === id)
    if (!found) throw new ApiError(404, 'not_found', 'Catatan tidak ditemukan.')
    return found
  }

  function findToken(id: string): Token {
    const found = state.tokens.find((row) => row.id === id)
    if (!found) throw new ApiError(404, 'not_found', 'Token tidak ditemukan.')
    return found
  }

  function revertToken(token: Token) {
    token.status = 'unused'
    token.linkedReadingId = null
    token.lockedAt = null
    token.canDelete = true
    token.deleteBlockedReason = null
    token.updatedAt = META()
  }

  function settingsView(): Settings {
    return {
      ...state.settings,
      tariff: {
        golongan: state.settings.tariffGolongan,
        perKwh: state.settings.tariffPerKwh,
      },
      pinSet: false,
      pushEnabled: false,
      importCompletedAt: null,
      updatedAt: META(),
    }
  }

  function checkReading(
    input: { reading: number; timestamp: string; rechargeAmount?: number | null; confirm?: string[] },
    ignoreId?: string,
  ) {
    const details: Array<{ code: string; message: string }> = []
    const confirmed = new Set(input.confirm ?? [])
    if (!confirmed.has('duplicate_timestamp')) {
      const duplicate = state.readings.some(
        (row) => row.id !== ignoreId && row.timestamp === input.timestamp,
      )
      if (duplicate)
        details.push({ code: 'duplicate_timestamp', message: 'Waktu catatan sudah dipakai.' })
    }
    if (!confirmed.has('increased_without_topup')) {
      const ms = Date.parse(input.timestamp)
      const previous = [...state.readings]
        .filter((row) => row.id !== ignoreId && Date.parse(row.timestamp) < ms)
        .sort((a, b) => Date.parse(b.timestamp) - Date.parse(a.timestamp))[0]
      if (previous && input.reading > previous.reading && (input.rechargeAmount ?? 0) <= 0)
        details.push({
          code: 'increased_without_topup',
          message: 'Angka naik tanpa isi ulang. Pastikan tidak salah ketik.',
        })
    }
    if (details.length > 0)
      throw new ApiError(409, 'confirmation_required', 'Perlu konfirmasi.', details)
  }

  function writeReading(id: string, input: Parameters<ApiClient['upsertReading']>[1]): Reading {
    checkReading(input, id)
    const now = META()
    const existing = state.readings.find((row) => row.id === id)
    const next: Reading = {
      id,
      meterName: input.meterName,
      reading: input.reading,
      timestamp: input.timestamp,
      notes: input.notes ?? null,
      rechargeAmount: input.rechargeAmount ?? null,
      cost: input.cost ?? null,
      tokenId: input.tokenId ?? null,
      source: input.source,
      locked: existing?.locked ?? false,
      oldId: input.oldId ?? existing?.oldId ?? null,
      createdAt: existing?.createdAt ?? now,
      updatedAt: now,
    }
    if (existing) Object.assign(existing, next)
    else state.readings.push(next)
    save()
    return next
  }

  function priceFor(kwh: number, cost: number): number {
    const list = [...state.settings.denominations].sort((a, b) => a - b)
    const nearest = list.reduce<number | null>(
      (best, value) => (best == null || Math.abs(value - cost) < Math.abs(best - cost) ? value : best),
      null,
    )
    return nearest == null ? Math.round(cost / kwh) : nearest
  }

  function deletable(token: Token | undefined): boolean {
  return token != null && token.status === 'unused' && !token.linkedReadingId
}

function writeToken(id: string, input: TokenUpsert): Token {
    const now = META()
    const existing = state.tokens.find((row) => row.id === id)
    if (existing?.status === 'used') {
      throw new ApiError(422, 'validation_error', 'Token yang sudah dipakai tidak bisa diubah.')
    }
    const denomination = input.denomination ?? null
    const cost = input.cost ?? null
    const nearest =
      denomination ??
      (cost == null || input.kwh <= 0 ? null : priceFor(input.kwh, cost))
    const next: Token = {
      id,
      kwh: input.kwh,
      cost,
      purchasedAt: input.purchasedAt,
      type: input.type,
      denomination: nearest,
      denominationMismatch:
        nearest == null || cost == null ? cost != null : Math.abs(cost - nearest) > nearest * 0.1,
      status: existing?.status ?? 'unused',
      source: existing?.source ?? 'manual',
      notes: input.notes ?? null,
      hasCode: existing?.hasCode ?? Boolean(input.code),
      linkedReadingId: existing?.linkedReadingId ?? null,
      canDelete: deletable(existing),
      deleteBlockedReason: deletable(existing)
        ? null
        : existing?.linkedReadingId
          ? 'refills_pending'
          : 'not_locked',
      lockedAt: existing?.lockedAt ?? null,
      refillsSinceLock: existing?.refillsSinceLock ?? null,
      createdAt: existing?.createdAt ?? now,
      updatedAt: now,
    }
    if (input.code) state.codes[id] = input.code
    if (existing) Object.assign(existing, next)
    else state.tokens.push(next)
    save()
    return next
  }

  return {
    mode: 'preview',

    async getSession() {
      return { authenticated: true, pinSet: false, expiresAt: null }
    },
    async login() {},
    async logout() {},
    async setPin() {
      unsupported('Penyimpanan PIN')
    },

    async listReadings(query: ReadingQuery = {}) {
      const from = query.from ? Date.parse(query.from) : null
      const to = query.to ? Date.parse(query.to) : null
      const rows = state.readings
        .filter((row) => (from == null || Date.parse(row.timestamp) >= from) && (to == null || Date.parse(row.timestamp) <= to))
        .sort((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp))
      return page(rows, query.limit)
    },
    async createReading(input) {
      return writeReading(input.id ?? newId('reading'), input)
    },
    async upsertReading(id, input) {
      return writeReading(id, input)
    },
    async deleteReading(id) {
      const row = findReading(id)
      if (row.locked) throw new ApiError(409, 'reading_locked', 'Catatan terkait token tidak bisa dihapus.')
      state.readings = state.readings.filter((item) => item.id !== id)
      for (const token of state.tokens) {
        if (token.linkedReadingId === id) revertToken(token)
      }
      save()
    },

    async listTokens(query: TokenQuery = {}) {
      const rows = state.tokens
        .filter((row) => !query.status || row.status === query.status)
        .sort((a, b) => Date.parse(b.purchasedAt) - Date.parse(a.purchasedAt))
      return page(rows, query.limit)
    },
    async createToken(input: TokenCreate) {
      return writeToken(input.id ?? newId('token'), { ...input })
    },
    async upsertToken(id, input) {
      return writeToken(id, input)
    },
    async patchToken(id, patch: TokenPatch) {
      const token = findToken(id)
      if (patch.type) token.type = patch.type
      if (patch.denomination !== undefined) token.denomination = patch.denomination
      if (patch.notes !== undefined) token.notes = patch.notes
      if (patch.purchasedAt) token.purchasedAt = patch.purchasedAt
      token.updatedAt = META()
      save()
      return token
    },
    async deleteToken(id) {
      const token = findToken(id)
      if (!token.canDelete) throw new ApiError(409, 'token_not_deletable', 'Token belum bisa dihapus.')
      state.tokens = state.tokens.filter((row) => row.id !== id)
      save()
    },
    async markTokenUsed(id, input) {
      const token = findToken(id)
      if (token.status !== 'unused')
        throw new ApiError(422, 'validation_error', 'Token tidak berstatus belum dipakai.')
      const reading = writeReading(newId('reading'), {
        meterName: 'main',
        reading: input.reading,
        timestamp: input.at,
        notes: input.notes ?? null,
        rechargeAmount: token.kwh,
        cost: token.cost,
        tokenId: token.id,
        source: 'token',
        confirm: input.confirm,
      })
      token.status = 'used'
      token.linkedReadingId = reading.id
      token.updatedAt = META()
      save()
      return { token, reading }
    },
    async cancelToken(id) {
      const token = findToken(id)
      if (token.status === 'locked')
        throw new ApiError(409, 'token_locked', 'Token terkunci tidak bisa dibatalkan.')
      if (token.status !== 'used')
        throw new ApiError(422, 'validation_error', 'Token belum dipakai.')
      const cancelledReadingId = token.linkedReadingId
      if (cancelledReadingId)
        state.readings = state.readings.filter((row) => row.id !== cancelledReadingId)
      revertToken(token)
      save()
      return { token, cancelledReadingId }
    },
    async revealTokenCode(id) {
      findToken(id)
      const code = state.codes[id]
      if (!code) throw new ApiError(404, 'not_found', 'Kode token tidak tersimpan.')
      return code
    },

    async getSettings() {
      return settingsView()
    },
    async patchSettings(patch: SettingsPatch) {
      if (patch.buffer != null) state.settings.buffer = patch.buffer
      if (patch.avgWindow) state.settings.avgWindow = patch.avgWindow
      if (patch.targetHour != null) state.settings.targetHour = patch.targetHour
      if (patch.reminderHour != null) state.settings.reminderHour = patch.reminderHour
      if (patch.denominations) state.settings.denominations = patch.denominations
      if (patch.tariff) {
        state.settings.tariffPerKwh = patch.tariff.perKwh
        state.settings.tariffGolongan = patch.tariff.golongan
      }
      save()
      return settingsView()
    },

    async getSummary(windowDays = state.settings.avgWindow) {
      const summary = summarize({
        readings: state.readings,
        tokens: state.tokens,
        buffer: state.settings.buffer,
        avgWindow: windowDays,
        nowMs: Date.now(),
        denominations: state.settings.denominations,
        targetHour: state.settings.targetHour,
        officialTariff: state.settings.tariffPerKwh,
      })
      return summaryToApi(summary, state.readings)
    },
    async getUsage(query: UsageQuery): Promise<UsagePage> {
      const fromMs = Math.max(Date.parse(query.from), monthStart(Date.now()) - 365 * 86_400_000)
      const buckets = usageBuckets(state.readings, {
        fromMs,
        toMs: Date.parse(query.to),
        group: query.group,
        price: priceOf(),
      }).map(usageBucketFromView)
      return {
        data: buckets,
        meta: {
          nextCursor: null,
          hasMore: false,
          total: buckets.length,
          totalKwh: buckets.reduce((sum, bucket) => sum + bucket.kwh, 0),
          totalCost: buckets.some((bucket) => bucket.cost == null)
            ? null
            : buckets.reduce((sum, bucket) => sum + (bucket.cost ?? 0), 0),
          group: query.group,
        },
      } satisfies UsagePage
    },
    async getHomepage(): Promise<Homepage> {
      const summary = summarize({
        readings: state.readings,
        tokens: state.tokens,
        buffer: state.settings.buffer,
        avgWindow: state.settings.avgWindow,
        nowMs: Date.now(),
        denominations: state.settings.denominations,
        targetHour: state.settings.targetHour,
      })
      return {
        label: 'kWh Tracker',
        value: summary.latest ? Date.parse(summary.latest.timestamp) / 1000 : null,
        textValue: `${summary.latest?.reading ?? 0} kWh`,
        targets: [
          {
            name: 'last_kwh',
            value: summary.latest?.reading ?? null,
            formatted: `${summary.latest?.reading ?? 0} kWh`,
          },
          {
            name: 'last_kwh_per_day',
            value: summary.avgKwh || null,
            formatted: `${summary.avgKwh.toFixed(2)} kWh/hari`,
          },
        ],
      }
    },

    async importPreview() {
      unsupported('Impor data')
    },
    async importCommit() {
      unsupported('Impor data')
    },

    async getReport() {
      unsupported('Unduh PDF')
    },

    async getPushKey() {
      unsupported('Notifikasi push')
    },
    async subscribePush() {
      unsupported('Notifikasi push')
    },
    async unsubscribePush() {
      unsupported('Notifikasi push')
    },
  }

  function priceOf(): number | null {
    const views = readingViews(state.readings)
    const priced = views.filter((row) => (row.rechargeAmount ?? 0) > 0 && (row.cost ?? 0) > 0)
    if (priced.length === 0) return null
    const totalKwh = priced.reduce((sum, row) => sum + (row.rechargeAmount ?? 0), 0)
    const totalCost = priced.reduce((sum, row) => sum + (row.cost ?? 0), 0)
    return totalKwh > 0 ? totalCost / totalKwh : null
  }
}

