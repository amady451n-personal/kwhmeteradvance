import type {
  Alert,
  Reading as ApiReading,
  Recommendation,
  TariffComparison,
  Token,
  Urgency,
} from '../api/types'

export type Reading = ApiReading
export type TokenRecord = Token
export type { Recommendation, Alert, TariffComparison, Urgency }

export interface DayUsage {
  key: string
  label: string
  kwh: number
  isToday: boolean
}

export type Comparison = 'hemat' | 'boros' | 'tetap'

export interface ReadingView extends Reading {
  delta: number | null
  rate: number | null
  comparison: Comparison | null
  comparisonDiff: number | null
  isTopup: boolean
}

export type DataQuality = 'ok' | 'insufficient'

export interface Summary {
  computedAt: number
  windowDays: number
  dataQuality: DataQuality
  latest: Reading | null
  latestMs: number | null
  avgWindow: number
  avgKwh: number
  todayKwh: number
  weekKwh: number
  weekCost: number | null
  monthKwh: number
  monthCost: number | null
  effectivePrice: number | null
  buffer: number
  tokenStock: number
  availableKwh: number
  usableKwh: number
  daysLeft: number | null
  depletionMs: number | null
  targetMs: number
  daysToTarget: number
  needKwh: number
  needRupiah: number | null
  recommendation: Recommendation | null
  tariffComparison: TariffComparison
  urgency: Urgency
  alerts: Alert[]
  chart: DayUsage[]
}