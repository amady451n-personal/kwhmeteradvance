import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { apiClient, ApiError } from '../api'
import { summaryFromApi } from '../api/mappers'
import {
  clearOffline,
  hasPinTrace,
  loadSnapshot,
  rememberPin,
  saveSnapshot,
  verifyPin,
} from '../api/offline.ts'
import {
  drainQueue,
  enqueueReading,
  enqueueToken,
  getPending,
  queueSize,
  removePending,
} from '../api/queue'
import type { DrainOutcome, QueuedToken } from '../api/queue'
import type { ReadingInput, Settings as ApiSettings, SettingsPatch, TokenCreate } from '../api'
import { DEFAULT_DENOMINATIONS, readingViews, summarize } from '../domain/energy'
import { formatDate, formatRelative } from '../domain/format'
import type { Reading, Summary, TokenRecord } from '../domain/types'

const PREVIEW_KEY = 'meteran.preview.dismissed'

interface Snapshot {
  readings: Reading[]
  tokens: TokenRecord[]
  settings: Settings
}

interface Settings {
  buffer: number
  avgWindow: 7 | 14 | 30
  targetHour: number
  reminderHour: number
  denominations: number[]
  tariffPerKwh: number | null
  tariffGolongan: string | null
  previewDismissed: boolean
}

export interface NewReading {
  timestamp: string
  reading: number
  rechargeAmount?: number | null
  cost?: number | null
  notes?: string | null
  confirm?: Array<'increased_without_topup' | 'duplicate_timestamp'>
}

export interface NewToken {
  code: string
  kwh: number
  cost: number
  purchasedAt: string
  type: 'additional' | 'bulk'
  denomination?: number | null
  notes?: string | null
  confirm?: Array<'denomination_out_of_tolerance' | 'duplicate_timestamp'>
}

const defaults: Settings = {
  buffer: 10,
  avgWindow: 7,
  targetHour: 6,
  reminderHour: 8,
  denominations: [...DEFAULT_DENOMINATIONS],
  tariffPerKwh: null,
  tariffGolongan: null,
  previewDismissed: localStorage.getItem(PREVIEW_KEY) === '1',
}

function applySettings(target: Settings, source: ApiSettings) {
  target.buffer = source.buffer
  target.avgWindow = source.avgWindow
  target.targetHour = source.targetHour
  target.reminderHour = source.reminderHour
  target.denominations = [...source.denominations]
  target.tariffPerKwh = source.tariff.perKwh
  target.tariffGolongan = source.tariff.golongan
}

export const useAppStore = defineStore('app', () => {
  const client = apiClient()
  const readings = ref<Reading[]>([])
  const tokens = ref<TokenRecord[]>([])
  const settings = reactive<Settings>({ ...defaults })
  const serverSummary = ref<Summary | null>(null)
  const nowMs = ref(Date.now())
  const loading = ref(true)
  const ready = ref(false)
  const offline = ref(false)
  const error = ref('')
  const queuedCount = ref(queueSize())
  const pendingTokens = ref<QueuedToken[]>([])
  const pendingCount = computed(() => pendingTokens.value.length)
  const session = ref<{ authenticated: boolean; pinSet: boolean } | null>(null)
  const toast = reactive({ open: false, message: '' })

  const summary = computed<Summary>(
    () =>
      serverSummary.value ??
      summarize({
        readings: readings.value,
        tokens: tokens.value,
        buffer: settings.buffer,
        avgWindow: settings.avgWindow,
        nowMs: nowMs.value,
        denominations: settings.denominations,
        targetHour: settings.targetHour,
        officialTariff: settings.tariffPerKwh,
      }),
  )
  const views = computed(() => readingViews(readings.value))
  const latest = computed(() => summary.value.latest)
  const stock = computed(() => summary.value.tokenStock)
  const sourceLabel = computed(() => {
    if (client.mode === 'preview') return 'Mode pratinjau perangkat'
    if (offline.value)
      return pendingCount.value ? `Data perangkat · ${pendingCount.value} tertunda` : 'Data perangkat'
    return 'Server tersambung'
  })
  const inputStatus = computed(() => {
    if (!summary.value.latestMs)
      return { label: 'Belum ada catatan', color: 'error', level: 'late' as const }
    const hours = (nowMs.value - summary.value.latestMs) / 3_600_000
    const label = formatRelative(summary.value.latestMs, nowMs.value)
    if (hours < 24) return { label, color: 'success', level: 'fresh' as const }
    if (hours < 48) return { label, color: 'warning', level: 'due' as const }
    return { label, color: 'error', level: 'late' as const }
  })

  async function login(pin: string) {
    await client.login(pin)
    await rememberPin(pin)
    session.value = { authenticated: true, pinSet: session.value?.pinSet ?? true }
    await init()
  }

  async function lock() {
    const known = await client.getSession().catch(() => null)
    session.value = {
      authenticated: false,
      pinSet: known?.pinSet ?? session.value?.pinSet ?? true,
    }
  }

  async function logout() {
    await client.logout().catch(() => {})
    clearOffline()
    pendingTokens.value = []
    await lock()
  }

  async function setPin(currentPin: string | null, newPin: string) {
    await client.setPin(currentPin, newPin)
    await rememberPin(newPin)
    session.value = { authenticated: true, pinSet: true }
    await init()
  }

  function notify(message: string) {
    toast.message = message
    toast.open = true
  }

  function persistSnapshot() {
    if (client.mode !== 'server') return
    void saveSnapshot({
      readings: [...readings.value],
      tokens: [...tokens.value],
      settings: { ...settings, denominations: [...settings.denominations] },
    }).catch(() => {})
  }

  async function syncPending() {
    if (client.mode !== 'server') return
    const before = queueSize()
    const asOutcome = (caught: unknown): DrainOutcome => {
      if (caught instanceof ApiError && !caught.isOffline && caught.status < 500) {
        return { ok: false, retryable: false, message: caught.message }
      }
      return { ok: false, retryable: true, message: caught instanceof Error ? caught.message : 'Gagal' }
    }
    const remaining = await drainQueue(
      async (id, input) => {
        try {
          await client.upsertReading(id, input)
          return { ok: true }
        } catch (caught) {
          return asOutcome(caught)
        }
      },
      async (id, input) => {
        try {
          await client.createToken({ ...input, id })
          return { ok: true }
        } catch (caught) {
          return asOutcome(caught)
        }
      },
    ).catch(() => queueSize())
    queuedCount.value = remaining
    pendingTokens.value = await getPending()
    if (remaining !== before) persistSnapshot()
    if (remaining === 0 && offline.value) void init(true)
  }

  async function discardPending(id: string) {
    queuedCount.value = await removePending(id)
    pendingTokens.value = await getPending()
  }

  async function unlockOffline(pin: string): Promise<boolean> {
    if (!(await verifyPin(pin))) return false
    const data = await loadSnapshot<Snapshot>()
    if (!data) {
      error.value = 'Belum ada salinan di perangkat. Hubungkan sekali untuk mengisi.'
      return false
    }
    readings.value = Array.isArray(data.readings) ? data.readings : []
    tokens.value = Array.isArray(data.tokens) ? data.tokens : []
    if (data.settings) Object.assign(settings, data.settings)
    session.value = { authenticated: true, pinSet: true }
    offline.value = true
    loading.value = false
    ready.value = true
    pendingTokens.value = await getPending()
    queuedCount.value = queueSize()
    void init(true)
    return true
  }

  function tick() {
    nowMs.value = Date.now()
  }

  async function refreshSummary() {
    serverSummary.value = summaryFromApi(await client.getSummary(settings.avgWindow))
  }

  async function init(silent = false) {
    loading.value = true
    error.value = ''
    try {
      const [readingsPage, tokensPage, serverSettings, serverView] = await Promise.all([
        client.listReadings({ limit: 500 }),
        client.listTokens({ limit: 200 }),
        client.getSettings(),
        client.getSummary().catch(() => null),
      ])
      readings.value = readingsPage.data
      session.value = await client.getSession().catch(() => null)
      tokens.value = tokensPage.data
      applySettings(settings, serverSettings)
      serverSummary.value = serverView ? summaryFromApi(serverView) : null
      offline.value = false
      ready.value = true
      persistSnapshot()
      await syncPending()
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 401) {
        if (!silent) await lock()
      } else if (caught instanceof ApiError && caught.isOffline && hasPinTrace()) {
        offline.value = true
        if (!silent) await lock()
      } else {
        offline.value = caught instanceof ApiError && caught.isOffline
        error.value = caught instanceof Error ? caught.message : 'Gagal memuat data.'
      }
    } finally {
      loading.value = false
    }
  }

  function fail(caught: unknown): never {
    if (caught instanceof ApiError && caught.status === 401) {
      session.value = { authenticated: false, pinSet: session.value?.pinSet ?? true }
      void lock()
      throw caught
    }
    error.value = caught instanceof Error ? caught.message : 'Permintaan gagal.'
    if (caught instanceof ApiError && caught.isOffline) {
      offline.value = true
      error.value = 'Tidak ada koneksi ke server.'
    }
    throw caught
  }

  function payload(input: NewReading, id?: string): ReadingInput {
    return {
      id,
      meterName: 'main',
      reading: input.reading,
      timestamp: input.timestamp,
      notes: input.notes ?? null,
      rechargeAmount: input.rechargeAmount ?? null,
      cost: input.cost ?? null,
      source: 'manual',
      confirm: input.confirm,
    }
  }

  function localReading(id: string, input: NewReading): Reading {
    const now = new Date().toISOString()
    const existing = readings.value.find((row) => row.id === id)
    return {
      id,
      meterName: 'main',
      reading: input.reading,
      timestamp: input.timestamp,
      notes: input.notes ?? null,
      rechargeAmount: input.rechargeAmount ?? null,
      cost: input.cost ?? null,
      tokenId: null,
      source: 'manual',
      locked: existing?.locked ?? false,
      oldId: null,
      createdAt: existing?.createdAt ?? now,
      updatedAt: now,
    }
  }

  async function keepQueued(id: string, input: NewReading) {
    queuedCount.value = await enqueueReading(id, payload(input, id))
    offline.value = true
    const next = localReading(id, input)
    const index = readings.value.findIndex((row) => row.id === id)
    if (index >= 0) readings.value[index] = next
    else readings.value.push(next)
    serverSummary.value = null
    notify('Tidak ada koneksi. Catatan menunggu antrean.')
  }

  async function addReading(input: NewReading) {
    const id = crypto.randomUUID()
    try {
      const created = await client.upsertReading(id, payload(input, id))
      readings.value.push(created)
    } catch (caught) {
      if (caught instanceof ApiError && caught.isOffline) {
        await keepQueued(id, input)
        return
      }
      return fail(caught)
    }
    serverSummary.value = null
    notify('Catatan meter tersimpan.')
    persistSnapshot()
    return refreshSummary().catch(() => {})
  }

  async function updateReading(id: string, input: NewReading) {
    try {
      const updated = await client.upsertReading(id, payload(input, id))
      const index = readings.value.findIndex((row) => row.id === id)
      if (index >= 0) readings.value[index] = updated
    } catch (caught) {
      if (caught instanceof ApiError && caught.isOffline) {
        await keepQueued(id, input)
        return
      }
      return fail(caught)
    }
    serverSummary.value = null
    notify('Catatan meter diperbarui.')
    persistSnapshot()
    return refreshSummary().catch(() => {})
  }

  async function deleteReading(id: string) {
    const removed = readings.value.find((row) => row.id === id)
    try {
      await client.deleteReading(id)
      readings.value = readings.value.filter((row) => row.id !== id)
      if (removed?.tokenId) tokens.value = (await client.listTokens({ limit: 200 })).data
    } catch (caught) {
      return fail(caught)
    }
    serverSummary.value = null
    notify('Catatan meter dihapus.')
    persistSnapshot()
    return refreshSummary().catch(() => {})
  }

  async function addToken(input: NewToken) {
    const body: TokenCreate = { ...input }
    try {
      const created = await client.createToken(body)
      tokens.value.unshift(created)
    } catch (caught) {
      if (caught instanceof ApiError && caught.isOffline) {
        const id = crypto.randomUUID()
        queuedCount.value = await enqueueToken(id, body)
        pendingTokens.value = await getPending()
        offline.value = true
        serverSummary.value = null
        notify('Token menunggu antrean.')
        return
      }
      return fail(caught)
    }
    serverSummary.value = null
    notify('Token tersimpan.')
    persistSnapshot()
    return refreshSummary().catch(() => {})
  }

  async function patchToken(id: string, patch: Partial<Pick<TokenRecord, 'type' | 'denomination' | 'notes' | 'purchasedAt'>>) {
    try {
      const updated = await client.patchToken(id, patch)
      const index = tokens.value.findIndex((row) => row.id === id)
      if (index >= 0) tokens.value[index] = updated
      return updated
    } catch (caught) {
      return fail(caught)
    }
  }

  async function markTokenUsed(id: string, reading = latest.value?.reading ?? 0) {
    try {
      const result = await client.markTokenUsed(id, { reading, at: new Date().toISOString() })
      const index = tokens.value.findIndex((row) => row.id === id)
      if (index >= 0) tokens.value[index] = result.token
      readings.value.push(result.reading)
    } catch (caught) {
      return fail(caught)
    }
    serverSummary.value = null
    notify('Token ditandai terpakai.')
    return refreshSummary().catch(() => {})
  }

  async function cancelToken(id: string) {
    let result
    try {
      result = await client.cancelToken(id)
    } catch (caught) {
      return fail(caught)
    }
    const index = tokens.value.findIndex((row) => row.id === id)
    if (index >= 0) tokens.value[index] = result.token
    if (result.cancelledReadingId) {
      readings.value = readings.value.filter((row) => row.id !== result.cancelledReadingId)
    }
    serverSummary.value = null
    notify('Token dikembalikan ke belum dipakai.')
    return refreshSummary().catch(() => {})
  }

  async function removeToken(id: string) {
    try {
      await client.deleteToken(id)
      tokens.value = tokens.value.filter((token) => token.id !== id)
    } catch (caught) {
      return fail(caught)
    }
    serverSummary.value = null
    notify('Token dihapus.')
    return refreshSummary().catch(() => {})
  }

  async function revealTokenCode(id: string) {
    try {
      return await client.revealTokenCode(id)
    } catch (caught) {
      return fail(caught)
    }
  }

  async function updateSettings(patch: Partial<Settings>) {
    Object.assign(settings, patch)
    const body: SettingsPatch = {}
    if (patch.buffer != null) body.buffer = patch.buffer
    if (patch.avgWindow) body.avgWindow = patch.avgWindow
    if (patch.targetHour != null) body.targetHour = patch.targetHour
    if (patch.reminderHour != null) body.reminderHour = patch.reminderHour
    if (patch.denominations) body.denominations = patch.denominations
    if (patch.tariffPerKwh !== undefined || patch.tariffGolongan !== undefined) {
      body.tariff = {
        perKwh: settings.tariffPerKwh,
        golongan: settings.tariffGolongan,
      }
    }
    try {
      applySettings(settings, await client.patchSettings(body))
      serverSummary.value = null
      await refreshSummary()
      persistSnapshot()
    } catch (caught) {
      fail(caught)
    }
  }

  function dismissPreview() {
    settings.previewDismissed = true
    localStorage.setItem(PREVIEW_KEY, '1')
  }

  return {
    mode: client.mode,
    readings,
    tokens,
    settings,
    summary,
    views,
    latest,
    stock,
    nowMs,
    loading,
    ready,
    offline,
    queuedCount,
    pendingTokens,
    pendingCount,
    syncPending,
    unlockOffline,
    discardPending,
    session,
    login,
    logout,
    setPin,
    error,
    toast,
    sourceLabel,
    inputStatus,
    tick,
    notify,
    init,
    refreshSummary,
    addReading,
    updateReading,
    deleteReading,
    addToken,
    patchToken,
    markTokenUsed,
    cancelToken,
    removeToken,
    revealTokenCode,
    updateSettings,
    dismissPreview,
    snapshotDate: formatDate(new Date().toISOString()),
  }
})