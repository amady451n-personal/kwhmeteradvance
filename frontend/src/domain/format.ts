const TZ = 'Asia/Jakarta'
const WIB_OFFSET_MS = 7 * 3_600_000

const dateTimeParts = new Intl.DateTimeFormat('id-ID', {
  day: '2-digit',
  month: 'short',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
  timeZone: TZ,
})

function partOf(parts: Intl.DateTimeFormatPart[], type: Intl.DateTimeFormatPartTypes): string {
  return parts.find((part) => part.type === type)?.value ?? ''
}

export function formatDateTime(value: string | number): string {
  const parts = dateTimeParts.formatToParts(new Date(value))
  return `${partOf(parts, 'day')} ${partOf(parts, 'month')} ${partOf(parts, 'year')}, ${partOf(parts, 'hour')}:${partOf(parts, 'minute')} WIB`
}

export function formatDate(value: string | number): string {
  const parts = dateTimeParts.formatToParts(new Date(value))
  return `${partOf(parts, 'day')} ${partOf(parts, 'month')} ${partOf(parts, 'year')}`
}

export function formatKwh(value: number | null | undefined, digits = 2): string {
  if (value == null || Number.isNaN(value)) return '-'
  return new Intl.NumberFormat('id-ID', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(value)
}

export function formatNumber(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '-'
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 0 }).format(value)
}

export function formatRupiah(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '-'
  return `Rp${new Intl.NumberFormat('id-ID', { maximumFractionDigits: 0 }).format(value)}`
}

export function formatRelative(value: string | number, nowMs: number): string {
  const then = new Date(value).getTime()
  const diff = Math.max(0, nowMs - then)
  const minutes = Math.floor(diff / 60_000)
  if (minutes < 2) return 'Baru saja'
  if (minutes < 60) return `${minutes} menit lalu`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} jam lalu`
  const days = Math.floor(hours / 24)
  if (days === 1) return '1 hari lalu'
  return `${days} hari lalu`
}

export function formatDays(value: number | null): string {
  if (value == null || Number.isNaN(value)) return 'Belum cukup data'
  if (value < 1) return 'Kurang dari 1 hari'
  return `${formatKwh(value, 1)} hari`
}

export function toWibInput(ms: number): string {
  return new Date(ms + WIB_OFFSET_MS).toISOString().slice(0, 16)
}

export function fromWibInput(value: string): string {
  return new Date(`${value}:00+07:00`).toISOString()
}

export function timeOnly(value: string | number): string {
  const parts = dateTimeParts.formatToParts(new Date(value))
  return `${partOf(parts, 'hour')}:${partOf(parts, 'minute')}`
}
