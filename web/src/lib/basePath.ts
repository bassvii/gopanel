// BASE_PATH берётся из <meta name="gopanel-base">, которое бэкенд
// подставляет при отдаче index.html. Так работает любой base_path,
// а не только hex-32 (старая регулярка отвергала /admin, /panel и т.п.).
//
// Fallback: если meta нет (Vite dev-сервер, тесты) — определяем
// из URL по старой эвристике.

export function getBasePath(): string {
  // 1. Основной путь: meta из index.html.
  if (typeof document !== 'undefined') {
    const meta = document.querySelector('meta[name="gopanel-base"]')
    const raw = meta?.getAttribute('content') ?? ''
    // __GOPANEL_BASE__ — placeholder, если бэкенд не подставил (dev).
    if (raw && raw !== '__GOPANEL_BASE__') {
      return raw.startsWith('/') ? raw : '/' + raw
    }
  }

  // 2. Fallback: старый способ через URL.
  const path = window.location.pathname
  const parts = path.split('/').filter(Boolean)
  if (parts.length === 0) return ''
  const first = parts[0]
  if (/^[a-f0-9]{16,}$/.test(first)) {
    return '/' + first
  }
  return ''
}
