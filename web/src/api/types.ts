export interface User {
  id: number
  name: string
  email: string
  enabled: boolean
  quota_bytes: number
  used_bytes: number
  expires_at: number
  device_limit: number
  sub_token_hash: string
  note: string
  created_at: string
}

export interface Inbound {
  id: number
  tag: string
  protocol: string
  port: number
  listen: string
  settings_json: string
  stream_json: string
  sniffing_json: string
  enabled: boolean
  created_at: string
}

export interface LinkEntry {
  inbound_id: number
  tag: string
  protocol: string
  remark: string
  url: string
  qr_base64?: string
  error?: string
}

export interface AuditEntry {
  id: number
  admin_id: number
  action: string
  action_ru: string
  target: string
  ip: string
  ts: number
}

export interface MeResponse {
  admin_id: number
  csrf_token: string
}

export interface LoginResponse {
  ok: boolean
  csrf_token?: string
  needs_totp?: boolean
  error?: string
}

export interface StatsSummary {
  users_total: number
  users_enabled: number
  inbounds_total: number
  inbounds_enabled: number
  traffic_total_bytes: number
  traffic_used_bytes: number
}

export interface Session {
  id: string
  ip: string
  user_agent: string
  last_seen: number
  expires_at: number
  current: boolean
}
