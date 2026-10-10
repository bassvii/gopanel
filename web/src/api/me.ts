import { request } from './client'
import type { Session } from './types'

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  await request('/me/password', {
    method: 'POST',
    body: { old_password: oldPassword, new_password: newPassword },
    withCSRF: true,
  })
}

export async function listSessions(): Promise<Session[]> {
  const res = await request<{ sessions: Session[] }>('/me/sessions')
  return res.sessions ?? []
}

export async function revokeSession(id: string): Promise<void> {
  await request(`/me/sessions/${id}`, { method: 'DELETE', withCSRF: true })
}

export async function revokeOtherSessions(): Promise<void> {
  await request('/me/sessions', { method: 'DELETE', withCSRF: true })
}
