import { getBasePath } from '../lib/basePath'

// downloadBackup скачивает дамп через браузер.
export function downloadBackup() {
  const base = getBasePath()
  const url = `${base}/backup/download`

  // fetch + blob → чтобы поддержать авторизацию.
  fetch(url, {
    credentials: 'same-origin',
  })
    .then((res) => {
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      return res.blob()
    })
    .then((blob) => {
      const link = document.createElement('a')
      link.href = URL.createObjectURL(blob)
      link.download = `gopanel-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.db`
      link.click()
      URL.revokeObjectURL(link.href)
    })
    .catch((err) => {
      alert('Ошибка загрузки бэкапа: ' + err.message)
    })
}
