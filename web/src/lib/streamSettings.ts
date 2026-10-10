// StreamSettings — удобное представление stream_json.
export interface StreamSettings {
  network: 'tcp' | 'ws' | 'grpc' | 'xhttp'
  security: 'none' | 'tls' | 'reality'

  // TCP
  tcpHeaderType: 'none' | 'http'

  // WebSocket
  wsPath: string
  wsHost: string

  // gRPC
  grpcServiceName: string

  // XHTTP
  xhttpPath: string
  xhttpHost: string
  xhttpMode: string

  // TLS
  tlsServerName: string
  tlsAlpn: string // запятая-разделённый список
  tlsFingerprint: string

  // REALITY
  realityDest: string
  realityServerNames: string // запятая-разделённый список
  realityPrivateKey: string
  realityShortIds: string // запятая-разделённый список
  realityFingerprint: string
  realityFlow: string // xtls-rprx-vision или пусто
  realitySpiderX: string
}

export const defaultStreamSettings: StreamSettings = {
  network: 'tcp',
  security: 'none',
  tcpHeaderType: 'none',
  wsPath: '/',
  wsHost: '',
  grpcServiceName: '',
  xhttpPath: '/',
  xhttpHost: '',
  xhttpMode: 'auto',
  tlsServerName: '',
  tlsAlpn: '',
  tlsFingerprint: 'chrome',
  realityDest: 'www.google.com:443',
  realityServerNames: 'www.google.com',
  realityPrivateKey: '',
  realityShortIds: '',
  realityFingerprint: 'chrome',
  realityFlow: '',
  realitySpiderX: '/',
}

// parseStreamJSON разбирает JSON-строку в структуру.
// Неизвестные поля игнорируются; отсутствующие заполняются значениями по умолчанию.
export function parseStreamJSON(json: string): StreamSettings {
  const out = { ...defaultStreamSettings }
  if (!json || json.trim() === '' || json.trim() === '{}') {
    return out
  }

  let raw: Record<string, unknown>
  try {
    raw = JSON.parse(json)
  } catch {
    return out
  }

  if (typeof raw.network === 'string') {
    const n = raw.network
    if (n === 'tcp' || n === 'ws' || n === 'grpc' || n === 'xhttp') {
      out.network = n
    }
  }

  if (typeof raw.security === 'string') {
    const s = raw.security
    if (s === 'none' || s === 'tls' || s === 'reality') {
      out.security = s
    }
  }

  // TCP header
  if (out.network === 'tcp') {
    const tcp = raw.tcpSettings as Record<string, unknown> | undefined
    if (tcp?.header) {
      const header = tcp.header as Record<string, unknown>
      if (header.type === 'http') out.tcpHeaderType = 'http'
      else out.tcpHeaderType = 'none'
    }
  }

  // WS
  if (out.network === 'ws') {
    const ws = raw.wsSettings as Record<string, unknown> | undefined
    if (typeof ws?.path === 'string') out.wsPath = ws.path
    if (typeof ws?.host === 'string') out.wsHost = ws.host
  }

  // gRPC
  if (out.network === 'grpc') {
    const grpc = raw.grpcSettings as Record<string, unknown> | undefined
    if (typeof grpc?.serviceName === 'string') out.grpcServiceName = grpc.serviceName
  }

  // XHTTP
  if (out.network === 'xhttp') {
    const xhttp = raw.xhttpSettings as Record<string, unknown> | undefined
    if (typeof xhttp?.path === 'string') out.xhttpPath = xhttp.path
    if (typeof xhttp?.host === 'string') out.xhttpHost = xhttp.host
    if (typeof xhttp?.mode === 'string') out.xhttpMode = xhttp.mode
  }

  // TLS
  if (out.security === 'tls') {
    const tls = raw.tlsSettings as Record<string, unknown> | undefined
    if (typeof tls?.serverName === 'string') out.tlsServerName = tls.serverName
    if (typeof tls?.fingerprint === 'string') out.tlsFingerprint = tls.fingerprint
    if (Array.isArray(tls?.alpn)) {
      out.tlsAlpn = (tls.alpn as unknown[]).filter((x) => typeof x === 'string').join(',')
    }
  }

  // REALITY
  if (out.security === 'reality') {
    const r = raw.realitySettings as Record<string, unknown> | undefined
    if (typeof r?.dest === 'string') out.realityDest = r.dest
    if (typeof r?.privateKey === 'string') out.realityPrivateKey = r.privateKey
    if (typeof r?.fingerprint === 'string') out.realityFingerprint = r.fingerprint
    if (typeof r?.flow === 'string') out.realityFlow = r.flow
    if (typeof r?.spiderX === 'string') out.realitySpiderX = r.spiderX
    if (Array.isArray(r?.serverNames)) {
      out.realityServerNames = (r.serverNames as unknown[])
        .filter((x) => typeof x === 'string')
        .join(',')
    }
    if (Array.isArray(r?.shortIds)) {
      out.realityShortIds = (r.shortIds as unknown[])
        .filter((x) => typeof x === 'string')
        .join(',')
    }
  }

  return out
}

// buildStreamJSON собирает JSON-строку из структуры.
// Всегда возвращает минимум {"network":"...","security":"..."}.
export function buildStreamJSON(s: StreamSettings): string {
  const out: Record<string, unknown> = {
    network: s.network,
    security: s.security,
  }

  // Транспорт-специфичные поля.
  switch (s.network) {
    case 'tcp':
      if (s.tcpHeaderType === 'http') {
        out.tcpSettings = { header: { type: 'http' } }
      }
      break
    case 'ws':
      out.wsSettings = {
        path: s.wsPath || '/',
        ...(s.wsHost ? { host: s.wsHost } : {}),
      }
      break
    case 'grpc':
      out.grpcSettings = {
        serviceName: s.grpcServiceName || '',
      }
      break
    case 'xhttp':
      out.xhttpSettings = {
        path: s.xhttpPath || '/',
        mode: s.xhttpMode || 'auto',
        ...(s.xhttpHost ? { host: s.xhttpHost } : {}),
      }
      break
  }

  // Безопасность.
  if (s.security === 'tls') {
    const tls: Record<string, unknown> = {}
    if (s.tlsServerName) tls.serverName = s.tlsServerName
    if (s.tlsFingerprint) tls.fingerprint = s.tlsFingerprint
    const alpn = s.tlsAlpn
      .split(',')
      .map((x) => x.trim())
      .filter(Boolean)
    if (alpn.length > 0) tls.alpn = alpn
    out.tlsSettings = tls
  }

  if (s.security === 'reality') {
    const serverNames = s.realityServerNames
      .split(',')
      .map((x) => x.trim())
      .filter(Boolean)
    const shortIds = s.realityShortIds
      .split(',')
      .map((x) => x.trim())
      .filter(Boolean)

    const reality: Record<string, unknown> = {
      dest: s.realityDest || 'www.google.com:443',
      serverNames: serverNames.length > 0 ? serverNames : ['www.google.com'],
      privateKey: s.realityPrivateKey,
      shortIds: shortIds.length > 0 ? shortIds : [''],
    }

    if (s.realityFingerprint) reality.fingerprint = s.realityFingerprint
    if (s.realityFlow) reality.flow = s.realityFlow
    if (s.realitySpiderX) reality.spiderX = s.realitySpiderX

    out.realitySettings = reality
  }

  return JSON.stringify(out, null, 2)
}
