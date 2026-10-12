import { request } from './client'

export interface Outbound {
  id: number
  tag: string
  protocol: string
  settings_json: string
  stream_json: string
  enabled: boolean
  is_builtin: boolean
  created_at: number
}

export interface OutboundInput {
  tag: string
  protocol: string
  settings_json: string
  stream_json: string
  enabled: boolean
}

export async function listOutbounds(): Promise<Outbound[]> {
  const res = await request<{ outbounds: Outbound[] }>('/outbounds')
  return res.outbounds ?? []
}

export async function createOutbound(input: OutboundInput): Promise<number> {
  const res = await request<{ id: number }>('/outbounds', {
    method: 'POST',
    body: input,
    withCSRF: true,
  })
  return res.id
}

export async function updateOutbound(id: number, input: OutboundInput): Promise<void> {
  await request(`/outbounds/${id}`, {
    method: 'PUT',
    body: input,
    withCSRF: true,
  })
}

export async function deleteOutbound(id: number): Promise<void> {
  await request(`/outbounds/${id}`, {
    method: 'DELETE',
    withCSRF: true,
  })
}
