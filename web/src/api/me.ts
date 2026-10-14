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


// --- 2FA (TOTP) ---

export interface TOTPSetupResponse {
  secret: string
  otpauth_url: string
}

export interface TOTPEnableResponse {
  ok: boolean
  recovery_codes?: string[]
  error?: string
}

export async function getTOTPStatus(): Promise<boolean> {
  const res = await request<{ enabled: boolean }>('/2fa/status')
  return res.enabled ?? false
}

export async function setupTOTP(): Promise<TOTPSetupResponse> {
  return request<TOTPSetupResponse>('/2fa/setup', {
    method: 'POST',
    withCSRF: true,
  })
}

export async function enableTOTP(code: string): Promise<TOTPEnableResponse> {
  return request<TOTPEnableResponse>('/2fa/enable', {
    method: 'POST',
    body: { code },
    withCSRF: true,
  })
}

export async function disableTOTP(password: string, code: string): Promise<void> {
  await request('/2fa/disable', {
    method: 'POST',
    body: { password, totp_code: code },
    withCSRF: true,
  })
}
