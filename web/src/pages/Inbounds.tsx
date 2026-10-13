import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Modal } from '../components/Modal'
import {
  listInbounds,
  createInbound,
  updateInbound,
  deleteInbound,
  type InboundInput,
} from '../api/inbounds'
import { ApiError } from '../api/client'
import {
  parseStreamJSON,
  buildStreamJSON,
  defaultStreamSettings,
  type StreamSettings,
} from '../lib/streamSettings'
import type { Inbound } from '../api/types'
import { toast } from 'sonner'
import { useConfirm } from '../hooks/useConfirm'
import { TableSkeleton } from '../components/Skeleton'
import { Plus, Pencil, Trash2, RefreshCw } from 'lucide-react'
import { Button, Badge, Toggle, Select } from '../components/ui'
import { generateX25519, generateShortID, generateVlessEnc } from '../api/xray'

// Пресеты протоколов — предзаполненные формы.
interface Preset {
  name: string
  protocol: string
  defaultPort: number
  settings: string
  stream: StreamSettings
  sniffing: string
}

const presets: Preset[] = [
  {
    name: 'VLESS',
    protocol: 'vless',
    defaultPort: 8443,
    settings: JSON.stringify({ decryption: 'none' }, null, 2),
    stream: { ...defaultStreamSettings, network: 'tcp', security: 'none' },
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'VLESS + REALITY',
    protocol: 'vless',
    defaultPort: 443,
    settings: JSON.stringify({ decryption: 'none' }, null, 2),
    stream: {
      ...defaultStreamSettings,
      network: 'tcp',
      security: 'reality',
      realityFlow: 'xtls-rprx-vision',
    },
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'VLESS + WS + TLS',
    protocol: 'vless',
    defaultPort: 443,
    settings: JSON.stringify({ decryption: 'none' }, null, 2),
    stream: {
      ...defaultStreamSettings,
      network: 'ws',
      security: 'tls',
      wsPath: '/ws',
      tlsFingerprint: 'chrome',
    },
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'VMess',
    protocol: 'vmess',
    defaultPort: 8443,
    settings: JSON.stringify({}, null, 2),
    stream: { ...defaultStreamSettings, network: 'tcp', security: 'none' },
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'Trojan',
    protocol: 'trojan',
    defaultPort: 443,
    settings: JSON.stringify({}, null, 2),
    stream: { ...defaultStreamSettings, network: 'tcp', security: 'tls' },
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'Shadowsocks',
    protocol: 'shadowsocks',
    defaultPort: 8388,
    settings: JSON.stringify({ method: 'aes-256-gcm' }, null, 2),
    stream: { ...defaultStreamSettings, network: 'tcp', security: 'none' },
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
]

export function Inbounds() {
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<Inbound | null>(null)
  const [creating, setCreating] = useState<Preset | null>(null)

  const { confirm, dialog } = useConfirm()

  async function reload() {
    setLoading(true)
    setError('')
    try {
      setInbounds(await listInbounds())
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка загрузки')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [])

  async function handleDelete(inb: Inbound) {
    const ok = await confirm({
      title: 'Удалить инбаунд?',
      message: `Инбаунд «${inb.tag}» будет удалён вместе со всеми связями с пользователями. Это действие нельзя отменить.`,
      confirmLabel: 'Удалить',
      danger: true,
    })
    if (!ok) return

    try {
      await deleteInbound(inb.id)
      await reload()
      toast.success('Инбаунд удалён')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка удаления')
    }
  }

  async function toggleEnabled(inb: Inbound) {
    try {
      await updateInbound(inb.id, {
        tag: inb.tag,
        protocol: inb.protocol,
        port: inb.port,
        listen: inb.listen,
        settings_json: inb.settings_json,
        stream_json: inb.stream_json,
        sniffing_json: inb.sniffing_json,
        enabled: !inb.enabled,
      })
      await reload()
      toast.success(inb.enabled ? 'Инбаунд отключён' : 'Инбаунд включён')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка переключения')
    }
  }

  return (
    <>
      <div className="flex-1 p-6 overflow-y-auto">
        {/* Заголовок */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-2xl font-bold text-white">Инбаунды</h2>
            <p className="text-sm mt-1" style={{ color: 'var(--text-muted)' }}>
              {inbounds.length}{' '}
              {inbounds.length === 1 ? 'запись' : inbounds.length < 5 ? 'записи' : 'записей'}
            </p>
          </div>
        </div>

        {/* Пресеты */}
        <div className="mb-6">
          <div
            className="text-xs uppercase tracking-wide mb-3"
            style={{ color: 'var(--text-dim)' }}
          >
            Создать из пресета
          </div>
          <div className="flex flex-wrap gap-2">
            {presets.map((p) => (
              <button
                key={p.name}
                onClick={() => setCreating(p)}
                className="text-sm px-3 py-2 rounded-lg transition-colors flex items-center gap-2"
                style={{
                  background: 'var(--bg-surface)',
                  border: '1px solid var(--border)',
                  color: 'var(--text-muted)',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = 'var(--bg-surface-hover)'
                  e.currentTarget.style.borderColor = 'var(--border-strong)'
                  e.currentTarget.style.color = 'var(--text)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = 'var(--bg-surface)'
                  e.currentTarget.style.borderColor = 'var(--border)'
                  e.currentTarget.style.color = 'var(--text-muted)'
                }}
              >
                <Plus size={14} />
                {p.name}
              </button>
            ))}
          </div>
        </div>

        {error && (
          <div
            className="mb-4 text-sm rounded-lg px-3 py-2"
            style={{
              color: 'var(--danger)',
              background: 'rgba(239, 68, 68, 0.1)',
              border: '1px solid rgba(239, 68, 68, 0.2)',
            }}
          >
            {error}
          </div>
        )}

        {loading ? (
          <TableSkeleton rows={4} cols={6} />
        ) : inbounds.length === 0 ? (
          <div className="card p-12 text-center">
            <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
              Инбаундов пока нет. Создайте первый из пресета выше.
            </p>
          </div>
        ) : (
          <div className="card overflow-hidden">
            <table className="table">
              <thead>
                <tr>
                  <th>Тег</th>
                  <th>Протокол</th>
                  <th>Порт</th>
                  <th>Listen</th>
                  <th>Статус</th>
                  <th style={{ textAlign: 'right' }}>Действия</th>
                </tr>
              </thead>
              <tbody>
                {inbounds.map((inb) => (
                  <tr key={inb.id}>
                    <td>
                      <span className="text-white font-medium">{inb.tag}</span>
                    </td>
                    <td>
                      <Badge variant="neutral">{inb.protocol}</Badge>
                    </td>
                    <td style={{ color: 'var(--text-muted)' }}>{inb.port}</td>
                    <td
                      className="font-mono text-xs"
                      style={{ color: 'var(--text-muted)' }}
                    >
                      {inb.listen}
                    </td>
                    <td>
                      <Toggle
                        enabled={inb.enabled}
                        onChange={() => toggleEnabled(inb)}
                      />
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setEditing(inb)}
                          icon={<Pencil size={14} />}
                        >
                          Изменить
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDelete(inb)}
                          icon={<Trash2 size={14} />}
                        >
                          Удалить
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {creating && (
        <InboundForm
          preset={creating}
          onClose={() => setCreating(null)}
          onSaved={async () => {
            setCreating(null)
            await reload()
          }}
        />
      )}

      {editing && (
        <InboundForm
          inbound={editing}
          onClose={() => setEditing(null)}
          onSaved={async () => {
            setEditing(null)
            await reload()
          }}
        />
      )}

      {dialog}
    </>
  )
}

// --- Форма ---

interface InboundFormProps {
  inbound?: Inbound
  preset?: Preset
  onClose: () => void
  onSaved: () => void
}

function InboundForm({ inbound, preset, onClose, onSaved }: InboundFormProps) {
  const isEdit = !!inbound

  const [tag, setTag] = useState(inbound?.tag ?? '')
  const [protocol, setProtocol] = useState(inbound?.protocol ?? preset?.protocol ?? 'vless')
  const [port, setPort] = useState(String(inbound?.port ?? preset?.defaultPort ?? 8443))
  const [listen, setListen] = useState(inbound?.listen ?? '0.0.0.0')
  const [settings, setSettings] = useState(inbound?.settings_json ?? preset?.settings ?? '{}')
  const [sniffing, setSniffing] = useState(inbound?.sniffing_json ?? preset?.sniffing ?? '{}')
  const [enabled, setEnabled] = useState(inbound?.enabled ?? true)

  const [stream, setStream] = useState<StreamSettings>(() => {
    if (inbound) return parseStreamJSON(inbound.stream_json)
    if (preset) return preset.stream
    return { ...defaultStreamSettings }
  })

  const [advanced, setAdvanced] = useState(false)
  const [advancedJSON, setAdvancedJSON] = useState('')
  const [generating, setGenerating] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const [realityPublicKey, setRealityPublicKey] = useState(
    inbound?.reality_public_key ?? ''
  )
  const [vlessEncServer, setVlessEncServer] = useState('')
  const [vlessEncClient, setVlessEncClient] = useState(
    inbound?.vless_enc_client ?? ''
  )

  async function handleGenerateVlessEnc() {
    setGenerating(true)
    setError('')
    try {
      const res = await generateVlessEnc()
      setVlessEncServer(res.server)
      setVlessEncClient(res.client)
      toast.success('VLESS Encryption сгенерирован')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка генерации')
    } finally {
      setGenerating(false)
    }
  }

  async function handleGenerateKeys() {
    setGenerating(true)
    setError('')
    try {
      const pair = await generateX25519()
      setStream((s) => ({ ...s, realityPrivateKey: pair.private_key }))
      setRealityPublicKey(pair.public_key)
      toast.success('Ключи сгенерированы')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка генерации ключей')
    } finally {
      setGenerating(false)
    }
  }

  async function handleGenerateShortID() {
    setGenerating(true)
    setError('')
    try {
      const id = await generateShortID()
      setStream((s) => ({
        ...s,
        realityShortIds: s.realityShortIds ? s.realityShortIds + ',' + id : id,
      }))
      toast.success('Short ID сгенерирован')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка генерации shortId')
    } finally {
      setGenerating(false)
    }
  }

  function openAdvanced() {
    setAdvancedJSON(buildStreamJSON(stream))
    setAdvanced(true)
  }

  function applyAdvanced() {
    try {
      const parsed = parseStreamJSON(advancedJSON)
      setStream(parsed)
      setAdvanced(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Некорректный JSON')
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSaving(true)

    const streamForSubmit = {
      ...stream,
      realityFlow: stream.realityFlow === '_none' ? '' : stream.realityFlow,
    }

    let finalSettings = settings
    if (protocol === 'vless' && vlessEncServer) {
      try {
        const parsed = JSON.parse(settings)
        parsed.decryption = vlessEncServer
        finalSettings = JSON.stringify(parsed, null, 2)
      } catch {
        // Игнорируем — settings невалидный JSON.
      }
    }

    const input: InboundInput = {
      tag,
      protocol,
      port: Number(port),
      listen,
      settings_json: finalSettings,
      stream_json: buildStreamJSON(streamForSubmit),
      sniffing_json: sniffing,
      reality_public_key: realityPublicKey,
      vless_enc_client: vlessEncClient,
      enabled,
    }

    try {
      if (isEdit && inbound) {
        await updateInbound(inbound.id, input)
        toast.success('Инбаунд обновлён')
      } else {
        await createInbound(input)
        toast.success('Инбаунд создан')
      }
      onSaved()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка сохранения')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      title={isEdit ? `Изменить «${inbound?.tag}»` : 'Новый инбаунд'}
      onClose={onClose}
      wide
    >
      <form onSubmit={handleSubmit} className="space-y-5">
        {/* Основное */}
        <div className="grid grid-cols-2 gap-4">
          <Field label="Тег">
            <input
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              required
              placeholder="vless-in"
              className={inputClass}
            />
          </Field>

          <div>
            <span className="text-sm text-neutral-400 block mb-1">Протокол</span>
            <Select
              value={protocol}
              onChange={setProtocol}
              options={[
                { value: 'vless', label: 'VLESS' },
                { value: 'vmess', label: 'VMess' },
                { value: 'trojan', label: 'Trojan' },
                { value: 'shadowsocks', label: 'Shadowsocks' },
                { value: 'socks', label: 'SOCKS' },
                { value: 'http', label: 'HTTP' },
              ]}
            />
          </div>

          <Field label="Порт">
            <input
              type="number"
              value={port}
              onChange={(e) => setPort(e.target.value)}
              min={1}
              max={65535}
              required
              className={inputClass}
            />
          </Field>

          <Field label="Listen">
            <input
              value={listen}
              onChange={(e) => setListen(e.target.value)}
              placeholder="0.0.0.0"
              className={inputClass}
            />
          </Field>
        </div>

        {/* Транспорт */}
        <Section title="Транспорт">
          <Radio
            value={stream.network}
            onChange={(v) => setStream({ ...stream, network: v as StreamSettings['network'] })}
            options={[
              { value: 'tcp', label: 'TCP' },
              { value: 'ws', label: 'WebSocket' },
              { value: 'grpc', label: 'gRPC' },
              { value: 'xhttp', label: 'XHTTP' },
            ]}
          />

          {stream.network === 'tcp' && (
            <div>
              <span className="text-sm text-neutral-400 block mb-1">HTTP-маскировка</span>
              <Select
                value={stream.tcpHeaderType}
                onChange={(v) => setStream({ ...stream, tcpHeaderType: v as 'none' | 'http' })}
                options={[
                  { value: 'none', label: 'Нет' },
                  { value: 'http', label: 'HTTP' },
                ]}
              />
            </div>
          )}

          {stream.network === 'ws' && (
            <div className="grid grid-cols-2 gap-4">
              <Field label="Path">
                <input
                  value={stream.wsPath}
                  onChange={(e) => setStream({ ...stream, wsPath: e.target.value })}
                  placeholder="/ws"
                  className={inputClass}
                />
              </Field>
              <Field label="Host (опционально)">
                <input
                  value={stream.wsHost}
                  onChange={(e) => setStream({ ...stream, wsHost: e.target.value })}
                  placeholder="example.com"
                  className={inputClass}
                />
              </Field>
            </div>
          )}

          {stream.network === 'grpc' && (
            <Field label="Service Name">
              <input
                value={stream.grpcServiceName}
                onChange={(e) => setStream({ ...stream, grpcServiceName: e.target.value })}
                placeholder="grpc-service"
                className={inputClass}
              />
            </Field>
          )}

          {stream.network === 'xhttp' && (
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-4">
                <Field label="Path">
                  <input
                    value={stream.xhttpPath}
                    onChange={(e) => setStream({ ...stream, xhttpPath: e.target.value })}
                    className={inputClass}
                  />
                </Field>
                <Field label="Host (опционально)">
                  <input
                    value={stream.xhttpHost}
                    onChange={(e) => setStream({ ...stream, xhttpHost: e.target.value })}
                    placeholder="example.com"
                    className={inputClass}
                  />
                </Field>
              </div>

              <div>
                <span className="text-sm text-neutral-400 block mb-1">Mode</span>
                <Select
                  value={stream.xhttpMode}
                  onChange={(v) => setStream({ ...stream, xhttpMode: v })}
                  options={[
                    { value: 'auto', label: 'auto' },
                    { value: 'packet-up', label: 'packet-up' },
                    { value: 'stream-up', label: 'stream-up' },
                    { value: 'stream-one', label: 'stream-one' },
                  ]}
                />
              </div>

              <label className="flex items-center gap-2 text-sm text-neutral-300">
                <input
                  type="checkbox"
                  checked={stream.xhttpExtraEnabled}
                  onChange={(e) => setStream({ ...stream, xhttpExtraEnabled: e.target.checked })}
                />
                Дополнительные параметры (для CDN и обхода детекции)
              </label>

              {stream.xhttpExtraEnabled && (
                <div className="border border-neutral-800 rounded p-4 space-y-3">
                  <Field label="xPaddingBytes (диапазон)">
                    <input
                      value={stream.xhttpPaddingBytes}
                      onChange={(e) => setStream({ ...stream, xhttpPaddingBytes: e.target.value })}
                      placeholder="100-1000"
                      className={inputClass + ' font-mono text-xs'}
                    />
                  </Field>

                  <Field label="scMaxEachPostBytes">
                    <input
                      value={stream.xhttpMaxPostBytes}
                      onChange={(e) => setStream({ ...stream, xhttpMaxPostBytes: e.target.value })}
                      placeholder="1000000"
                      className={inputClass + ' font-mono text-xs'}
                    />
                  </Field>

                  <Field label="scMaxBufferedPosts">
                    <input
                      type="number"
                      value={String(stream.xhttpMaxBuffered)}
                      onChange={(e) =>
                        setStream({ ...stream, xhttpMaxBuffered: Number(e.target.value) || 0 })
                      }
                      min={0}
                      className={inputClass + ' font-mono text-xs'}
                    />
                  </Field>

                  <Field label="scStreamUpServerSecs (диапазон)">
                    <input
                      value={stream.xhttpStreamUpSecs}
                      onChange={(e) => setStream({ ...stream, xhttpStreamUpSecs: e.target.value })}
                      placeholder="20-80"
                      className={inputClass + ' font-mono text-xs'}
                    />
                  </Field>

                  <label className="flex items-center gap-2 text-sm text-neutral-300">
                    <input
                      type="checkbox"
                      checked={stream.xhttpNoSSE}
                      onChange={(e) => setStream({ ...stream, xhttpNoSSE: e.target.checked })}
                    />
                    noSSEHeader (убрать заголовок text/event-stream)
                  </label>
                </div>
              )}
            </div>
          )}
        </Section>

        {/* Безопасность */}
        <Section title="Безопасность">
          <Radio
            value={stream.security}
            onChange={(v) => setStream({ ...stream, security: v as StreamSettings['security'] })}
            options={[
              { value: 'none', label: 'Нет' },
              { value: 'tls', label: 'TLS' },
              { value: 'reality', label: 'REALITY' },
            ]}
          />

          {stream.security === 'tls' && (
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-4">
                <Field label="Server Name (SNI)">
                  <input
                    value={stream.tlsServerName}
                    onChange={(e) => setStream({ ...stream, tlsServerName: e.target.value })}
                    placeholder="example.com"
                    className={inputClass}
                  />
                </Field>
                <div>
                  <span className="text-sm text-neutral-400 block mb-1">Fingerprint</span>
                  <Select
                    value={stream.tlsFingerprint}
                    onChange={(v) => setStream({ ...stream, tlsFingerprint: v })}
                    options={[
                      { value: 'chrome', label: 'chrome' },
                      { value: 'firefox', label: 'firefox' },
                      { value: 'safari', label: 'safari' },
                      { value: 'random', label: 'random' },
                    ]}
                  />
                </div>
              </div>
              <Field label="ALPN (через запятую)">
                <input
                  value={stream.tlsAlpn}
                  onChange={(e) => setStream({ ...stream, tlsAlpn: e.target.value })}
                  placeholder="h2,http/1.1"
                  className={inputClass}
                />
              </Field>
            </div>
          )}

          {stream.security === 'reality' && (
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-4">
                <Field label="Dest (сайт-донор)">
                  <input
                    value={stream.realityDest}
                    onChange={(e) => setStream({ ...stream, realityDest: e.target.value })}
                    placeholder="www.google.com:443"
                    className={inputClass}
                  />
                </Field>
                <Field label="Server Names (SNI, через запятую)">
                  <input
                    value={stream.realityServerNames}
                    onChange={(e) => setStream({ ...stream, realityServerNames: e.target.value })}
                    placeholder="www.google.com"
                    className={inputClass}
                  />
                </Field>
              </div>

              <Field label="Private Key">
                <div className="flex gap-2">
                  <input
                    value={stream.realityPrivateKey}
                    onChange={(e) => setStream({ ...stream, realityPrivateKey: e.target.value })}
                    placeholder="нажмите «Сгенерировать»"
                    className={inputClass + ' flex-1 font-mono text-xs'}
                  />
                  <button
                    type="button"
                    onClick={handleGenerateKeys}
                    disabled={generating}
                    className="btn btn-secondary"
                    style={{ whiteSpace: 'nowrap' }}
                  >
                    <RefreshCw size={14} />
                    {generating ? '...' : 'Сгенерировать'}
                  </button>
                </div>
              </Field>

              <Field label="Public Key (для ссылок)">
                <input
                  value={realityPublicKey}
                  readOnly
                  placeholder="генерируется вместе с Private Key"
                  className={inputClass + ' font-mono text-xs'}
                />
              </Field>

              <Field label="Short IDs (через запятую)">
                <div className="flex gap-2">
                  <input
                    value={stream.realityShortIds}
                    onChange={(e) => setStream({ ...stream, realityShortIds: e.target.value })}
                    placeholder="нажмите «Сгенерировать»"
                    className={inputClass + ' flex-1 font-mono text-xs'}
                  />
                  <button
                    type="button"
                    onClick={handleGenerateShortID}
                    disabled={generating}
                    className="btn btn-secondary"
                    style={{ whiteSpace: 'nowrap' }}
                  >
                    <RefreshCw size={14} />
                    {generating ? '...' : 'Сгенерировать'}
                  </button>
                </div>
              </Field>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <span className="text-sm text-neutral-400 block mb-1">Fingerprint</span>
                  <Select
                    value={stream.realityFingerprint}
                    onChange={(v) => setStream({ ...stream, realityFingerprint: v })}
                    options={[
                      { value: 'chrome', label: 'chrome' },
                      { value: 'firefox', label: 'firefox' },
                      { value: 'safari', label: 'safari' },
                      { value: 'random', label: 'random' },
                    ]}
                  />
                </div>
                <div>
                  <span className="text-sm text-neutral-400 block mb-1">Flow</span>
                  <Select
                    value={stream.realityFlow}
                    onChange={(v) => setStream({ ...stream, realityFlow: v })}
                    options={[
                      { value: '_none', label: '(нет)' },
                      { value: 'xtls-rprx-vision', label: 'xtls-rprx-vision' },
                    ]}
                  />
                </div>
              </div>
            </div>
          )}
        </Section>

        {/* VLESS Encryption */}
        {protocol === 'vless' && stream.security === 'none' && (
          <Section title="VLESS Encryption (опционально)">
            <div className="text-xs mb-3" style={{ color: 'var(--text-muted)' }}>
              Шифрование VLESS без TLS/REALITY. Нажмите «Сгенерировать» — строки подставятся в конфиг и ссылку.
            </div>

            <Button
              type="button"
              onClick={handleGenerateVlessEnc}
              disabled={generating}
              size="sm"
            >
              {generating ? 'Генерация...' : 'Сгенерировать'}
            </Button>

            {vlessEncServer && (
              <div className="space-y-3 mt-3">
                <Field label="Server (decryption)">
                  <input
                    value={vlessEncServer}
                    readOnly
                    className={inputClass + ' font-mono text-xs'}
                  />
                </Field>
              </div>
            )}
          </Section>
        )}

        {/* Расширенные настройки */}
        <div>
          {!advanced ? (
            <button
              type="button"
              onClick={openAdvanced}
              className="text-xs text-neutral-500 hover:text-white"
            >
              ▼ Расширенные настройки (JSON)
            </button>
          ) : (
            <div className="space-y-2">
              <button
                type="button"
                onClick={() => setAdvanced(false)}
                className="text-xs text-neutral-500 hover:text-white"
              >
                ▲ Скрыть расширенные настройки
              </button>
              <textarea
                value={advancedJSON}
                onChange={(e) => setAdvancedJSON(e.target.value)}
                rows={12}
                className={inputClass + ' font-mono text-xs'}
              />
              <button
                type="button"
                onClick={applyAdvanced}
                className="bg-neutral-800 hover:bg-neutral-700 text-white text-xs rounded px-3 py-1.5"
              >
                Применить JSON
              </button>
            </div>
          )}
        </div>

        {/* Settings и Sniffing */}
        <details className="text-sm">
          <summary className="text-neutral-500 cursor-pointer hover:text-white">
            Settings и Sniffing (JSON)
          </summary>
          <div className="space-y-3 mt-3">
            <Field label="Settings (JSON)">
              <textarea
                value={settings}
                onChange={(e) => setSettings(e.target.value)}
                rows={4}
                className={inputClass + ' font-mono text-xs'}
              />
            </Field>
            <Field label="Sniffing (JSON)">
              <textarea
                value={sniffing}
                onChange={(e) => setSniffing(e.target.value)}
                rows={3}
                className={inputClass + ' font-mono text-xs'}
              />
            </Field>
          </div>
        </details>

        <label className="flex items-center gap-2 text-sm text-neutral-300">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          Включён
        </label>

        {error && <div className="text-sm text-red-400">{error}</div>}

        <div className="flex justify-end gap-2 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="text-sm text-neutral-400 hover:text-white px-4 py-2"
          >
            Отмена
          </button>
          <button
            type="submit"
            disabled={saving}
            className="bg-white text-black font-medium text-sm rounded px-4 py-2 hover:bg-neutral-200 disabled:opacity-50"
          >
            {saving ? 'Сохранение...' : 'Сохранить'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

// --- Вспомогательные компоненты ---

const inputClass =
  'w-full bg-neutral-950 border border-neutral-800 rounded-lg px-3 py-2 text-white outline-none focus:border-[color:var(--accent)] transition-colors'

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="text-sm text-neutral-400 block mb-1">{label}</span>
      {children}
    </label>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="border border-neutral-800 rounded p-4 space-y-3">
      <div className="text-sm text-neutral-400 font-medium">{title}</div>
      {children}
    </div>
  )
}

function Radio({
  value,
  onChange,
  options,
}: {
  value: string
  onChange: (v: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {options.map((o) => {
        const active = value === o.value
        return (
          <button
            key={o.value}
            type="button"
            onClick={() => onChange(o.value)}
            className="text-sm px-3 py-2 transition-colors"
            style={{
              borderRadius: 8,
              background: active ? 'var(--accent)' : 'var(--bg-surface)',
              border: active ? '1px solid var(--accent)' : '1px solid var(--border)',
              color: active ? '#ffffff' : 'var(--text-muted)',
              fontWeight: active ? 500 : 400,
            }}
            onMouseEnter={(e) => {
              if (!active) {
                e.currentTarget.style.background = 'var(--bg-surface-hover)'
                e.currentTarget.style.borderColor = 'var(--border-strong)'
                e.currentTarget.style.color = 'var(--text)'
              }
            }}
            onMouseLeave={(e) => {
              if (!active) {
                e.currentTarget.style.background = 'var(--bg-surface)'
                e.currentTarget.style.borderColor = 'var(--border)'
                e.currentTarget.style.color = 'var(--text-muted)'
              }
            }}
          >
            {o.label}
          </button>
        )
      })}
    </div>
  )
}
