import { request } from './client'
import type { AuditEntry } from './types'

export async function getAudit(limit = 100): Promise<AuditEntry[]> {
  const res = await request<{ entries: AuditEntry[] }>(`/audit?limit=${limit}`)
  return res.entries ?? []
}
