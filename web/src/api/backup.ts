import { getBasePath } from '../lib/basePath'
import { getCSRF } from './client'

export async function downloadBackup(password: string): Promise<Blob> {
  const base = getBasePath()
  const url = `${base}/backup/download`

  const res = await fetch(url, {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': getCSRF(),
    },
    body: JSON.stringify({ password }),
  })

  if (!res.ok) {
    let message = `HTTP ${res.status}`
    try {
      const data = await res.json()
      if (data && typeof data.error === 'string') {
        message = data.error
      }
    } catch {
      /* не JSON — оставим HTTP-код */
    }
    throw new Error(message)
  }

  return res.blob()
}

export function saveBlob(blob: Blob, filename: string) {
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}

export function defaultBackupName(): string {
  const now = new Date()
  const stamp = now.toISOString().slice(0, 19).replace(/[:T]/g, '-')
  return `gopanel-${stamp}.db`
}
