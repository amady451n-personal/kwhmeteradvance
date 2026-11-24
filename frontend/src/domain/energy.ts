import type {
  Alert,
  DataQuality,
  DayUsage,
  Reading,
  ReadingView,
  Recommendation,
  Summary,
  TariffComparison,
  TokenRecord,
  Urgency,
} from './types'

export const DAY_MS = 86_400_000
export const WIB_OFFSET_MS = 7 * 3_600_000
export const DEFAULT_DENOMINATIONS = [20_000, 50_000, 100_000, 200_000, 500_000]

export function toMs(value: string): number {
  return new Date(value).getTime()
}

export function dayKey(ms: number): string {
  return new Date(ms + WIB_OFFSET_MS).toISOString().slice(0, 10)
}

export function dayStart(ms: number): number {
  return Math.floor((ms + WIB_OFFSET_MS) / DAY_MS) * DAY_MS - WIB_OFFSET_MS
}

export function dayLabel(key: string): string {
  return new Intl.DateTimeFormat('id-ID', {
    weekday: 'short',
    timeZone: 'Asia/Jakarta',
  }).format(new Date(`${key}T00:00:00+07:00`))
}

export function sortReadings(readings: Reading[]): Reading[] {
  return [...readings].sort(
    (a, b) => toMs(a.timestamp) - toMs(b.timestamp) || a.id.localeCompare(b.id),
  )
}

export interface Interval {
  from: Reading
  to: Reading
  startMs: number
  endMs: number
  hours: number
  consumption: number
  isTopup: boolean
}

export function buildIntervals(readings: Reading[]): Interval[] {
  const sorted = sortReadings(readings)
  const result: Interval[] = []

  for (let index = 1; index < sorted.length; index += 1) {
    const from = sorted[index - 1]
    const to = sorted[index]
    const startMs = toMs(from.timestamp)
    const endMs = toMs(to.timestamp)
    const hours = (endMs - startMs) / 3_600_000
    const topup = to.rechargeAmount ?? 0
    const consumption = from.reading + topup - to.reading

    if (hours <= 0 || consumption < 0) continue

    result.push({
      from,
      to,
      startMs,
      endMs,
      hours,
      consumption,
      isTopup: topup > 0,
    })
  }

  return result
}

export function dailyUsage(readings: Reading[]): Map<string, number> {
  const daily = new Map<string, number>()

  for (const interval of buildIntervals(readings)) {
    if (interval.consumption <= 0) continue
    const span = interval.endMs - interval.startMs
    let cursor = interval.startMs

    while (cursor < interval.endMs) {
      const boundary = dayStart(cursor) + DAY_MS
      const segmentEnd = Math.min(boundary, interval.endMs)
      const share = interval.consumption * ((segmentEnd - cursor) / span)
      const key = dayKey(cursor)
      daily.set(key, (daily.get(key) ?? 0) + share)
      cursor = segmentEnd
    }
  }

  return daily
}

export function windowUsage(daily: Map<string, number>, nowMs: number, days: number): number {
  let total = 0
  const today = dayStart(nowMs)
  for (let index = 0; index < days; index += 1) {
    total += daily.get(dayKey(today - index * DAY_MS)) ?? 0
  }
  return total
}

export function nextTargetMs(nowMs: number, hour = 6): number {
  const local = new Date(nowMs + WIB_OFFSET_MS)
  const year = local.getUTCFullYear()
  const month = local.getUTCMonth()
  const thisMonth = Date.UTC(year, month, 1, hour) - WIB_OFFSET_MS
  if (nowMs < thisMonth) return thisMonth
  return Date.UTC(year, month + 1, 1, hour) - WIB_OFFSET_MS
}

export function effectivePrice(readings: Reading[]): number | null {
  let kwh = 0
  let cost = 0

  for (const reading of readings) {
    const recharge = reading.rechargeAmount ?? 0
    const paid = reading.cost ?? 0
    if (recharge > 0 && paid > 1) {
      kwh += recharge
      cost += paid
    }
  }

  return kwh > 0 ? cost / kwh : null
}

export function recommendToken(
  needKwh: number,
  price: number | null,
  denominations: number[],
): Recommendation | null {
  if (needKwh <= 0) return null
  if (!price || price <= 0) {
    return { kwh: needKwh, rupiah: null, denomination: null, status: 'price_unknown' }
  }

  const needRupiah = needKwh * price
  const sorted = denominations.filter((value) => value > 0).sort((a, b) => a - b)
  if (sorted.length === 0) {
    return {
      kwh: needKwh,
      rupiah: null,
      denomination: null,
      status: 'no_denomination_available',
    }
  }

  const exact = sorted.find((value) => value >= needRupiah)
  if (exact) return { kwh: exact / price, rupiah: exact, denomination: exact, status: 'ok' }

  const largest = sorted[sorted.length - 1]
  return {
    kwh: largest / price,
    rupiah: largest,
    denomination: largest,
    status: 'largest_only',
  }
}

export function monthStart(ms: number): number {
  const local = new Date(ms + WIB_OFFSET_MS)
  return Date.UTC(local.getUTCFullYear(), local.getUTCMonth(), 1) - WIB_OFFSET_MS
}

export function monthNext(ms: number): number {
  const local = new Date(ms + WIB_OFFSET_MS)
  return Date.UTC(local.getUTCFullYear(), local.getUTCMonth() + 1, 1) - WIB_OFFSET_MS
}

export function weekStart(ms: number): number {
  const start = dayStart(ms)
  const weekday = new Date(start + WIB_OFFSET_MS).getUTCDay()
  return start - ((weekday + 6) % 7) * DAY_MS
}

export function tariffComparison(
  purchasePricePerKwh: number | null,
  officialTariffPerKwh: number | null,
): TariffComparison {
  if (
    purchasePricePerKwh == null ||
    officialTariffPerKwh == null ||
    officialTariffPerKwh <= 0
  ) {
    return { purchasePricePerKwh, officialTariffPerKwh, diffRupiah: null, diffPercent: null }
  }

  const diffRupiah = purchasePricePerKwh - officialTariffPerKwh
  return {
    purchasePricePerKwh,
    officialTariffPerKwh,
    diffRupiah,
    diffPercent: (diffRupiah / officialTariffPerKwh) * 100,
  }
}

export function urgencyFor(latestMs: number | null, nowMs: number): Urgency {
  if (!latestMs) {
    return { level: 'overdue', hoursSinceLastInput: null, message: 'Belum ada catatan meter.' }
  }

  const hours = Math.max(0, (nowMs - latestMs) / 3_600_000)
  if (hours < 24) {
    return { level: 'neutral', hoursSinceLastInput: hours, message: 'Catatan terakhir masih baru.' }
  }
  if (hours <= 40) {
    return {
      level: 'remind',
      hoursSinceLastInput: hours,
      message: 'Catat sekarang agar pemakaian tetap terpantau.',
    }
  }
  return {
    level: 'overdue',
    hoursSinceLastInput: hours,
    message: 'Perlu input! Sisa kWh tidak terpantau.',
  }
}

interface AlertInput {
  dataQuality: DataQuality
  daysLeft: number | null
  urgency: Urgency
  tokenStock: number
  recommendation: Recommendation | null
  denominations: number[]
}

function buildAlerts(input: AlertInput): Alert[] {
  const alerts: Alert[] = []
  if (input.dataQuality === 'insufficient') {
    alerts.push({
      code: 'data_insufficient',
      severity: 'info',
      message: 'Butuh minimal dua catatan untuk memprediksi pemakaian.',
    })
  }
  if (input.urgency.level === 'remind') {
    alerts.push({ code: 'input_overdue', severity: 'warning', message: input.urgency.message })
  }
  if (input.urgency.level === 'overdue') {
    alerts.push({ code: 'input_overdue', severity: 'critical', message: input.urgency.message })
  }
  if (input.daysLeft != null && input.daysLeft <= 3) {
    alerts.push({
      code: 'buffer_near',
      severity: input.daysLeft <= 1 ? 'critical' : 'warning',
      message: 'Sisa kWh mendekati buffer.',
    })
  }
  if (input.recommendation != null && input.tokenStock <= 0) {
    alerts.push({ code: 'topup_reminder', severity: 'warning', message: 'Belum ada token tersimpan.' })
  }
  if (input.denominations.length === 0) {
    alerts.push({
      code: 'denomination_missing',
      severity: 'info',
      message: 'Daftar denominasi kosong, saran token tanpa nominal.',
    })
  }
  return alerts
}

export interface SummaryInput {
  readings: Reading[]
  tokens?: TokenRecord[]
  buffer: number
  avgWindow: number
  nowMs: number
  denominations?: number[]
  targetHour?: number
  officialTariff?: number | null
}

export function summarize(input: SummaryInput): Summary {
  const nowMs = input.nowMs
  const daily = dailyUsage(input.readings)
  const sorted = sortReadings(input.readings)
  const latest = sorted.length > 0 ? sorted[sorted.length - 1] : null
  const todayKey = dayKey(nowMs)
  const monthKey = todayKey.slice(0, 7)
  const avgTotal = windowUsage(daily, nowMs, input.avgWindow)
  const avgKwh = avgTotal / Math.max(1, input.avgWindow)
  const tokenStock = (input.tokens ?? [])
    .filter((token) => token.status === 'unused' && token.type === 'additional')
    .reduce((total, token) => total + token.kwh, 0)
  const availableKwh = (latest?.reading ?? 0) + tokenStock
  const usableKwh = Math.max(0, availableKwh - input.buffer)
  const daysLeft = latest && avgKwh > 0 ? usableKwh / avgKwh : null
  const targetMs = nextTargetMs(nowMs, input.targetHour ?? 6)
  const daysToTarget = (targetMs - nowMs) / DAY_MS
  const needKwh = latest ? Math.max(0, avgKwh * daysToTarget + input.buffer - availableKwh) : 0
  const price = effectivePrice(input.readings)
  const denominations = input.denominations ?? DEFAULT_DENOMINATIONS
  const dataQuality: DataQuality =
    latest && sorted.length >= 2 && avgKwh > 0 ? 'ok' : 'insufficient'

  let monthKwh = 0
  for (const [key, value] of daily) {
    if (key.startsWith(monthKey)) monthKwh += value
  }

  const startWeek = weekStart(nowMs)
  let weekKwh = 0
  for (let cursor = startWeek; cursor <= dayStart(nowMs); cursor += DAY_MS) {
    weekKwh += daily.get(dayKey(cursor)) ?? 0
  }

  const recommendation =
    dataQuality === 'ok' ? recommendToken(needKwh, price, denominations) : null
  const urgency = urgencyFor(latest ? toMs(latest.timestamp) : null, nowMs)
  const needRupiah = recommendation?.rupiah ?? (price == null ? null : needKwh * price)

  const chart: DayUsage[] = []
  const today = dayStart(nowMs)
  for (let index = 6; index >= 0; index -= 1) {
    const key = dayKey(today - index * DAY_MS)
    chart.push({
      key,
      label: dayLabel(key),
      kwh: daily.get(key) ?? 0,
      isToday: key === todayKey,
    })
  }

  return {
    computedAt: nowMs,
    windowDays: input.avgWindow,
    dataQuality,
    latest,
    latestMs: latest ? toMs(latest.timestamp) : null,
    avgWindow: input.avgWindow,
    avgKwh,
    todayKwh: daily.get(todayKey) ?? 0,
    weekKwh,
    weekCost: price == null ? null : weekKwh * price,
    monthKwh,
    monthCost: price == null ? null : monthKwh * price,
    effectivePrice: price,
    buffer: input.buffer,
    tokenStock,
    availableKwh,
    usableKwh,
    daysLeft,
    depletionMs: daysLeft == null ? null : nowMs + daysLeft * DAY_MS,
    targetMs,
    daysToTarget,
    needKwh,
    needRupiah,
    recommendation,
    tariffComparison: tariffComparison(price, input.officialTariff ?? null),
    urgency,
    alerts: buildAlerts({
      dataQuality,
      daysLeft,
      urgency,
      tokenStock,
      recommendation,
      denominations,
    }),
    chart,
  }
}

export interface UsageBucketView {
  key: string
  label: string
  startMs: number
  endMs: number
  kwh: number
  cost: number | null
  estimated: boolean
  estimatedDays: number
  days: number
}

const shortDate = new Intl.DateTimeFormat('id-ID', {
  day: 'numeric',
  month: 'short',
  timeZone: 'Asia/Jakarta',
})
const shortMonth = new Intl.DateTimeFormat('id-ID', {
  month: 'short',
  year: 'numeric',
  timeZone: 'Asia/Jakarta',
})

function bucketStart(ms: number, group: 'day' | 'week' | 'month'): number {
  if (group === 'day') return dayStart(ms)
  if (group === 'week') return weekStart(ms)
  return monthStart(ms)
}

function bucketNext(ms: number, group: 'day' | 'week' | 'month'): number {
  if (group === 'day') return ms + DAY_MS
  if (group === 'week') return ms + 7 * DAY_MS
  return monthNext(ms)
}

export function usageBuckets(
  readings: Reading[],
  options: {
    fromMs: number
    toMs: number
    group: 'day' | 'week' | 'month'
    price: number | null
  },
): UsageBucketView[] {
  const daily = dailyUsage(readings)
  const ownDays = new Set(readings.map((reading) => dayKey(toMs(reading.timestamp))))
  const buckets: UsageBucketView[] = []
  const { fromMs, group, price } = options
  const endMs = options.toMs

  let start = bucketStart(fromMs, group)
  while (start < endMs) {
    const end = Math.min(bucketNext(start, group), endMs)
    let kwh = 0
    let days = 0
    let estimatedDays = 0

    for (let cursor = Math.max(start, dayStart(fromMs)); cursor < end; cursor += DAY_MS) {
      const key = dayKey(cursor)
      const value = daily.get(key) ?? 0
      kwh += value
      days += 1
      if (value > 0 && !ownDays.has(key)) estimatedDays += 1
    }

    const key =
      group === 'day'
        ? dayKey(start)
        : group === 'month'
          ? dayKey(start).slice(0, 7)
          : `${dayKey(start)}..${dayKey(end - 1)}`

    const labelStart = group === 'week' ? Math.max(start, dayStart(fromMs)) : start
    buckets.push({
      key,
      label:
        group === 'day'
          ? shortDate.format(new Date(start))
          : group === 'month'
            ? shortMonth.format(new Date(start))
            : `${shortDate.format(new Date(labelStart))} - ${shortDate.format(new Date(end - 1))}`,
      startMs: start,
      endMs: end,
      kwh,
      cost: price == null ? null : kwh * price,
      estimated: estimatedDays > 0,
      estimatedDays,
      days,
    })

    start = bucketNext(start, group)
  }

  return buckets
}
export function readingViews(readings: Reading[]): ReadingView[] {
  const sorted = sortReadings(readings)
  const views: ReadingView[] = sorted.map((reading) => ({
    ...reading,
    delta: null,
    rate: null,
    comparison: null,
    comparisonDiff: null,
    isTopup: (reading.rechargeAmount ?? 0) > 0,
  }))
  const rates: Array<number | null> = views.map(() => null)

  for (let index = 1; index < views.length; index += 1) {
    const previous = views[index - 1]
    const current = views[index]
    if (current.isTopup) continue

    const hours = (toMs(current.timestamp) - toMs(previous.timestamp)) / 3_600_000
    if (hours <= 0) continue

    const delta = previous.reading - current.reading
    if (delta < 0) continue

    current.delta = delta
    current.rate = (delta / hours) * 24
    rates[index] = current.rate
  }

  for (let index = 1; index < views.length; index += 1) {
    const current = views[index]
    const previous = views[index - 1]
    if (current.isTopup || previous.isTopup) continue

    const currentRate = rates[index]
    const previousRate = rates[index - 1]
    if (currentRate == null || previousRate == null) continue

    const diff = currentRate - previousRate
    current.comparisonDiff = Math.abs(diff)
    if (Math.abs(diff) <= previousRate * 0.1) current.comparison = 'tetap'
    else current.comparison = diff < 0 ? 'hemat' : 'boros'
  }

  return views.reverse()
}
