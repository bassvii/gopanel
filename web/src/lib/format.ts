export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 Б'
  const units = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const val = bytes / Math.pow(1024, i)
  return `${val.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatDate(unix: number): string {
  if (!unix || unix <= 0) return '—'
  const d = new Date(unix * 1000)
  return d.toLocaleDateString('ru-RU', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

export function unixFromDateInput(value: string): number {
  if (!value) return 0
  return Math.floor(new Date(value + 'T00:00:00').getTime() / 1000)
}

export function dateInputFromUnix(unix: number): string {
  if (!unix || unix <= 0) return ''
  const d = new Date(unix * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function capitalize(s: string): string {
  if (!s) return s
  return s.charAt(0).toUpperCase() + s.slice(1)
}
