const dateTimeFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
})

const relativeFormat = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })

export function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return dateTimeFormat.format(date)
}

/** "just now", "3 minutes ago", then falls back to a date. */
export function formatRelative(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  const seconds = (date.getTime() - Date.now()) / 1000
  const absolute = Math.abs(seconds)

  if (absolute < 45) return 'just now'
  if (absolute < 60 * 60) return relativeFormat.format(Math.round(seconds / 60), 'minute')
  if (absolute < 60 * 60 * 24) return relativeFormat.format(Math.round(seconds / 3600), 'hour')
  if (absolute < 60 * 60 * 24 * 7) return relativeFormat.format(Math.round(seconds / 86400), 'day')
  return formatDateTime(value)
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value)
}
