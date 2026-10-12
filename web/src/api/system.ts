import { request } from './client'

export interface SystemCurrent {
  cpu_percent: number
  cpu_cores: number
  mem_used_bytes: number
  mem_total_bytes: number
  mem_percent: number
  disk_used_bytes: number
  disk_total_bytes: number
  disk_percent: number
  load_avg_1: number
  load_avg_5: number
  load_avg_15: number
  uptime_seconds: number
  ts: number
}

export interface SystemPoint {
  ts: number
  cpu_percent: number
  mem_percent: number
  disk_percent: number
  load_avg_1: number
}

export interface XrayStatus {
  running: boolean
  version: string
  uptime_seconds: number
}

export interface PanelStatus {
  version: string
  uptime_seconds: number
  db_size_bytes: number
}

export interface SystemStats {
  current: SystemCurrent
  history: SystemPoint[]
  xray: XrayStatus
  panel: PanelStatus
}

export async function getSystemStats(): Promise<SystemStats> {
  return request<SystemStats>('/system/stats')
}
