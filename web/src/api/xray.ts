import { request } from './client'

export interface X25519Pair {
  private_key: string
  public_key: string
}

export async function generateX25519(): Promise<X25519Pair> {
  return request<X25519Pair>('/xray/x25519', {
    method: 'POST',
    withCSRF: true,
  })
}

export interface ShortIDResult {
  short_id: string
}

export async function generateShortID(): Promise<string> {
  const res = await request<ShortIDResult>('/xray/shortid', {
    method: 'POST',
    withCSRF: true,
  })
  return res.short_id
}

export interface VlessEncResult {
  server: string
  client: string
}

export async function generateVlessEnc(): Promise<VlessEncResult> {
  return request<VlessEncResult>('/xray/vlessenc', {
    method: 'POST',
    withCSRF: true,
  })
}

export interface TLSCertRequest {
  domain: string
  name?: string
  expire_hours?: number
}

export interface TLSCertResult {
  cert_path: string
  key_path: string
  domain: string
}

export async function generateTLSCert(req: TLSCertRequest): Promise<TLSCertResult> {
  return request<TLSCertResult>('/xray/tls-cert', {
    method: 'POST',
    body: req,
    withCSRF: true,
  })
}
