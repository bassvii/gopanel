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
