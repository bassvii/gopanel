import { request } from './client'
import type { Inbound } from './types'

export async function listInbounds(): Promise<Inbound[]> {
  const res = await request<{ inbounds: Inbound[] }>('/inbounds')
  return res.inbounds ?? []
}

export interface InboundInput {
  tag: string
  protocol: string
  port: number
  listen?: string
  settings_json?: string
  stream_json?: string
  sniffing_json?: string
  enabled: boolean
}

export async function createInbound(input: InboundInput): Promise<number> {
  const res = await request<{ id: number }>('/inbounds', {
    method: 'POST',
    body: input,
    withCSRF: true,
  })
  return res.id
}

export async function updateInbound(id: number, input: InboundInput): Promise<void> {
  await request(`/inbounds/${id}`, {
    method: 'PUT',
    body: input,
    withCSRF: true,
  })
}

export async function deleteInbound(id: number): Promise<void> {
  await request(`/inbounds/${id}`, {
    method: 'DELETE',
    withCSRF: true,
  })
}
