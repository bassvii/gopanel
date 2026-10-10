import { getBasePath } from '../lib/basePath'

let csrfToken = ''

export function setCSRF(token: string) {
  csrfToken = token
}

export function getCSRF(): string {
  return csrfToken
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  body?: unknown
  withCSRF?: boolean
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const base = getBasePath()
  const url = `${base}${path}`
  const method = opts.method ?? 'GET'

  const headers: Record<string, string> = {}
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  if (opts.withCSRF && csrfToken) {
    headers['X-CSRF-Token'] = csrfToken
  }

  const res = await fetch(url, {
    method,
    headers,
    credentials: 'same-origin',
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })

  if (res.status === 401) {
    // Сессия истекла — редирект на логин.
    throw new ApiError(401, 'unauthorized')
  }

  const text = await res.text()
  let data: unknown = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }

  if (!res.ok) {
    const msg =
      typeof data === 'object' && data !== null && 'error' in data
        ? String((data as Record<string, unknown>).error)
        : res.statusText
    throw new ApiError(res.status, msg)
  }

  return data as T
}
