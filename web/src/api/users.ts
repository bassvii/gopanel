import { request } from './client'
import type { LinkEntry, User } from './types'

export async function listUsers(): Promise<User[]> {
  const res = await request<{ users: User[] }>('/users')
  return res.users ?? []
}

export interface CreateUserInput {
  name: string
  email?: string
  enabled: boolean
  quota_bytes?: number
  expires_at?: number
  device_limit?: number
  note?: string
}

export async function createUser(input: CreateUserInput): Promise<number> {
  const res = await request<{ id: number }>('/users', {
    method: 'POST',
    body: input,
    withCSRF: true,
  })
  return res.id
}

export async function updateUser(id: number, input: Partial<CreateUserInput>): Promise<void> {
  await request(`/users/${id}`, {
    method: 'PUT',
    body: input,
    withCSRF: true,
  })
}

export async function deleteUser(id: number): Promise<void> {
  await request(`/users/${id}`, {
    method: 'DELETE',
    withCSRF: true,
  })
}

export async function attachUserToInbound(userId: number, inboundId: number): Promise<string> {
  const res = await request<{ credential_json: string }>(`/users/${userId}/inbounds`, {
    method: 'POST',
    body: { inbound_id: inboundId },
    withCSRF: true,
  })
  return res.credential_json
}

export async function getUserLinks(userId: number, address: string, withQR = false): Promise<LinkEntry[]> {
  const qs = new URLSearchParams({ address })
  if (withQR) qs.set('qr', '1')
  const res = await request<{ links: LinkEntry[] }>(`/users/${userId}/links?${qs}`)
  return res.links ?? []
}
