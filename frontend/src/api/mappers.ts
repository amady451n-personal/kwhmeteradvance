import {
  dayKey,
  toMs,
  WIB_OFFSET_MS,
} from '../domain/energy'
import type { UsageBucketView } from '../domain/energy'
import type { DayUsage, Summary } from '../domain/types'
import type {
  Reading,
  Summary as ApiSummary,
  UsageBucket,
} from './types'

export function wibIso(ms: number): string {
  return new Date(ms + WIB_OFFSET_MS).toISOString().replace('Z', '+07:00')
}

export function wibIsoOrNull(ms: number | null): string | null {
  return ms == null ? null : wibIso(ms)
}

export function dayUsageFromBucket(bucket: UsageBucket): DayUsage {
  return {
    key: bucket.key,
    label: bucket.label,
    kwh: bucket.kwh,
    isToday: bucket.key === dayKey(Date.now()),
  }
}

export function usageBucketViewFromApi(bucket: UsageBucket): UsageBucketView {
  return {
    key: bucket.key,
    label: bucket.label,
    startMs: toMs(bucket.start),
    endMs: toMs(bucket.end),
    kwh: bucket.kwh,
    cost: bucket.cost,
    estimated: bucket.estimated,
    estimatedDays: bucket.estimatedDays,
    days: bucket.days,
  }
}

export function usageBucketFromView(view: UsageBucketView): UsageBucket {
  return {
    key: view.key,
    label: view.label,
    start: wibIso(view.startMs),
    end: wibIso(view.endMs),
    kwh: view.kwh,
    cost: view.cost,
    estimated: view.estimated,
    estimatedDays: view.estimatedDays,
    days: view.days,
  }
}

/**
 * Internal view model to contract shape. Preview mode stands in for the
 * server, so it must answer with the same payload the Go backend will send.
 */
export function summaryToApi(summary: Summary, readings: Reading[]): ApiSummary {
  const readingDays = new Set(readings.map((reading) => dayKey(toMs(reading.timestamp))))

  return {
    computedAt: wibIso(summary.computedAt),
    windowDays: summary.windowDays as ApiSummary['windowDays'],
    dataQuality: summary.dataQuality,
    latestReading: summary.latest,
    avgKwhPerDay: summary.avgKwh,
    todayKwh: summary.todayKwh,
    weekKwh: summary.weekKwh,
    weekCost: summary.weekCost,
    monthKwh: summary.monthKwh,
    monthCost: summary.monthCost,
    effectivePricePerKwh: summary.effectivePrice,
    buffer: summary.buffer,
    tokenStockKwh: summary.tokenStock,
    availableKwh: summary.availableKwh,
    usableKwh: summary.usableKwh,
    daysLeft: summary.daysLeft,
    depletionAt: wibIsoOrNull(summary.depletionMs),
    targetAt: wibIso(summary.targetMs),
    needKwh: summary.needKwh,
    needRupiah: summary.needRupiah,
    recommendation: summary.recommendation,
    tariffComparison: summary.tariffComparison,
    urgency: summary.urgency,
    alerts: summary.alerts,
    chart: summary.chart.map((day) => ({
      key: day.key,
      label: day.label,
      start: wibIso(toMs(`${day.key}T00:00:00+07:00`)),
      end: wibIso(toMs(`${day.key}T00:00:00+07:00`) + 86_400_000),
      kwh: day.kwh,
      cost: summary.effectivePrice == null ? null : day.kwh * summary.effectivePrice,
      estimated: day.kwh > 0 && !readingDays.has(day.key),
      estimatedDays: day.kwh > 0 && !readingDays.has(day.key) ? 1 : 0,
      days: 1,
    })),
  }
}

export function summaryFromApi(api: ApiSummary): Summary {
  return {
    computedAt: toMs(api.computedAt),
    windowDays: api.windowDays,
    dataQuality: api.dataQuality,
    latest: api.latestReading,
    latestMs: api.latestReading ? toMs(api.latestReading.timestamp) : null,
    avgWindow: api.windowDays,
    avgKwh: api.avgKwhPerDay ?? 0,
    todayKwh: api.todayKwh ?? 0,
    weekKwh: api.weekKwh ?? 0,
    weekCost: api.weekCost,
    monthKwh: api.monthKwh ?? 0,
    monthCost: api.monthCost,
    effectivePrice: api.effectivePricePerKwh,
    buffer: api.buffer,
    tokenStock: api.tokenStockKwh,
    availableKwh: api.availableKwh,
    usableKwh: api.usableKwh,
    daysLeft: api.daysLeft,
    depletionMs: api.depletionAt == null ? null : toMs(api.depletionAt),
    targetMs: toMs(api.targetAt),
    daysToTarget: (toMs(api.targetAt) - Date.now()) / 86_400_000,
    needKwh: api.needKwh,
    needRupiah: api.needRupiah,
    recommendation: api.recommendation,
    tariffComparison: api.tariffComparison,
    urgency: api.urgency,
    alerts: api.alerts,
    chart: api.chart.map(dayUsageFromBucket),
  }
}