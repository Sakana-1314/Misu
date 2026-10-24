import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

export function formatTime(value?: string | number | Date | null): string {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'number') {
    if (value > 1e12) return dayjs(value).format('YYYY-MM-DD HH:mm:ss')
    if (value > 0) return dayjs.unix(value).format('YYYY-MM-DD HH:mm:ss')
    return '—'
  }
  const d = dayjs(value)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : '—'
}

export function fromUnix(value?: number | null): string {
  if (!value || value <= 0) return '—'
  return dayjs.unix(value).format('YYYY-MM-DD HH:mm:ss')
}

export function dateOnly(value?: string | number | Date | null): string {
  if (!value) return '—'
  return dayjs(value).format('YYYY-MM-DD')
}

export function relativeTimeText(value?: string | number | null): string {
  if (!value) return '—'
  return dayjs(value).fromNow()
}

export function formatDuration(seconds?: number | null): string {
  if (!seconds || seconds <= 0) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d} 天 ${h} 小时`
  if (h > 0) return `${h} 小时 ${m} 分钟`
  return `${m} 分钟`
}

export function formatSecondsAgo(seconds?: number | null): string {
  if (seconds === null || seconds === undefined || seconds < 0) return '—'
  if (seconds < 60) return `${seconds} 秒前`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`
  return `${Math.floor(seconds / 86400)} 天前`
}

export function prettyJSON(value: any): string {
  if (value === null || value === undefined) return '—'
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

export function truncate(value: string, limit = 60): string {
  if (!value) return ''
  return value.length > limit ? `${value.slice(0, limit)}…` : value
}

export function parseLines(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

export function percent(done: number, total: number): number {
  if (!total) return 0
  return Math.max(0, Math.min(100, Math.round((done / total) * 100)))
}
