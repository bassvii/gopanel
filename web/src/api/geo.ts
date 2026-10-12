import { request } from './client'

export interface GeoFileInfo {
  name: string
  path: string
  exists: boolean
  size_bytes: number
  modified_at: number
}

export interface GeoStatus {
  dir: string
  geosite: GeoFileInfo
  geoip: GeoFileInfo
}

export async function getGeoStatus(): Promise<GeoStatus> {
  return request<GeoStatus>('/system/geo/status')
}

export interface GeoUpdateResult {
  ok: boolean
  updated: number
  status: GeoStatus
}

export async function updateGeo(): Promise<GeoUpdateResult> {
  return request<GeoUpdateResult>('/system/geo/update', {
    method: 'POST',
    withCSRF: true,
  })
}
