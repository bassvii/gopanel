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
