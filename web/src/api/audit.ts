import { request } from './client'
import type { AuditEntry } from './types'

export interface AuditResponse {
  entries: AuditEntry[]
  total: number
  limit: number
  offset: number
}

export async function getAudit(limit = 100, offset = 0, q = ''): Promise<AuditResponse> {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  if (q) qs.set('q', q)
  return request<AuditResponse>(`/audit?${qs}`)
}
