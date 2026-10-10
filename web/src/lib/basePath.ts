// BASE_PATH берётся из URL: первый сегмент пути.
// Например, /c7f75b36.../users → /c7f75b36...
// Для Vite dev-сервера (localhost:5173) — пусто.
export function getBasePath(): string {
  const path = window.location.pathname
  const parts = path.split('/').filter(Boolean)
  if (parts.length === 0) return ''
  // Первый сегмент — секретный base path (hex, 32 символа).
  // Если он похож на хеш, берём его.
  const first = parts[0]
  if (/^[a-f0-9]{16,}$/.test(first)) {
    return '/' + first
  }
  return ''
}
