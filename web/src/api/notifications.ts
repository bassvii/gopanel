import { request } from './client'

export interface Notification {
  id: number
  category: string
  severity: 'info' | 'warning' | 'error'
  title: string
  body: string
  read_at: number
  created_at: number
}

export interface NotificationsResponse {
  notifications: Notification[]
  unread: number
}

export async function listNotifications(limit = 20): Promise<NotificationsResponse> {
  return request<NotificationsResponse>(`/notifications?limit=${limit}`)
}

export async function markAllRead(): Promise<number> {
  const res = await request<{ ok: boolean; unread: number }>('/notifications/read', {
    method: 'POST',
    body: { all: true },
    withCSRF: true,
  })
  return res.unread
}

export async function markRead(id: number): Promise<number> {
  const res = await request<{ ok: boolean; unread: number }>('/notifications/read', {
    method: 'POST',
    body: { id },
    withCSRF: true,
  })
  return res.unread
}
