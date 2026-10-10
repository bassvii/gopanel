import { request, setCSRF } from './client'
import type { LoginResponse, MeResponse } from './types'

export async function login(username: string, password: string, totpCode?: string): Promise<LoginResponse> {
  const res = await request<LoginResponse>('/login', {
    method: 'POST',
    body: { username, password, totp_code: totpCode },
  })
  if (res.ok && res.csrf_token) {
    setCSRF(res.csrf_token)
  }
  return res
}

export async function logout(): Promise<void> {
  await request('/logout', { method: 'POST' })
  setCSRF('')
}

export async function me(): Promise<MeResponse> {
  const res = await request<MeResponse>('/me')
  setCSRF(res.csrf_token)
  return res
}
