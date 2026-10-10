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
import { generateX25519, generateShortID } from '../api/xray'
import {
  parseStreamJSON,
  buildStreamJSON,
  defaultStreamSettings,
  type StreamSettings,
} from '../lib/streamSettings'
import type { Inbound } from '../api/types'

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
    if (!confirm(`Удалить инбаунд «${inb.tag}»?`)) return
    try {
      await deleteInbound(inb.id)
      await reload()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Ошибка удаления')
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
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Ошибка переключения')
    }
  }

  return (
    <div className="flex-1 p-6 overflow-y-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold text-white">Инбаунды</h2>
          <p className="text-sm text-neutral-500">
            {inbounds.length} {inbounds.length === 1 ? 'запись' : 'записей'}
          </p>
        </div>
      </div>

      <div className="mb-6">
        <div className="text-sm text-neutral-400 mb-2">Создать из пресета:</div>
        <div className="flex flex-wrap gap-2">
          {presets.map((p) => (
            <button
              key={p.name}
              onClick={() => setCreating(p)}
              className="text-sm bg-neutral-800 hover:bg-neutral-700 text-white rounded px-3 py-1.5"
            >
              + {p.name}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <div className="mb-4 text-sm text-red-400 bg-red-950/30 border border-red-900 rounded px-3 py-2">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-neutral-500">Загрузка...</div>
      ) : inbounds.length === 0 ? (
        <div className="text-neutral-500 text-center py-16">
          Инбаундов пока нет. Создайте первый из пресета выше.
        </div>
      ) : (
        <div className="border border-neutral-800 rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-neutral-950 text-neutral-400">
              <tr>
                <th className="text-left px-4 py-3 font-medium">Тег</th>
                <th className="text-left px-4 py-3 font-medium">Протокол</th>
                <th className="text-left px-4 py-3 font-medium">Порт</th>
                <th className="text-left px-4 py-3 font-medium">Listen</th>
                <th className="text-left px-4 py-3 font-medium">Статус</th>
                <th className="text-right px-4 py-3 font-medium">Действия</th>
              </tr>
            </thead>
            <tbody>
              {inbounds.map((inb) => (
                <tr key={inb.id} className="border-t border-neutral-800 hover:bg-neutral-900/40">
                  <td className="px-4 py-3 text-white">{inb.tag}</td>
                  <td className="px-4 py-3 text-neutral-400">{inb.protocol}</td>
                  <td className="px-4 py-3 text-neutral-400">{inb.port}</td>
                  <td className="px-4 py-3 text-neutral-400">{inb.listen}</td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => toggleEnabled(inb)}
                      className={
                        'text-xs px-2 py-1 rounded ' +
                        (inb.enabled
                          ? 'text-green-400 hover:bg-green-950/30'
                          : 'text-red-400 hover:bg-red-950/30')
                      }
                    >
                      {inb.enabled ? 'включён' : 'отключён'}
                    </button>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => setEditing(inb)}
                      className="text-neutral-400 hover:text-white text-xs mr-3"
                    >
                      Изменить
                    </button>
                    <button
                      onClick={() => handleDelete(inb)}
                      className="text-red-400 hover:text-red-300 text-xs"
                    >
                      Удалить
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

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
    </div>
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

  // Stream settings — структура вместо сырого JSON.
  const [stream, setStream] = useState<StreamSettings>(() => {
    if (inbound) return parseStreamJSON(inbound.stream_json)
    if (preset) return preset.stream
    return { ...defaultStreamSettings }
  })

  // Показать расширенные настройки (сырой JSON).
  const [advanced, setAdvanced] = useState(false)
  const [advancedJSON, setAdvancedJSON] = useState('')

  // Кнопки генерации.
  const [generating, setGenerating] = useState(false)

  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleGenerateKeys() {
    setGenerating(true)
    setError('')
    try {
      const pair = await generateX25519()
      setStream((s) => ({ ...s, realityPrivateKey: pair.private_key }))
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

    const input: InboundInput = {
      tag,
      protocol,
      port: Number(port),
      listen,
      settings_json: settings,
      stream_json: buildStreamJSON(stream),
      sniffing_json: sniffing,
      enabled,
    }

    try {
      if (isEdit && inbound) {
        await updateInbound(inbound.id, input)
      } else {
        await createInbound(input)
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

          <Field label="Протокол">
            <select
              value={protocol}
              onChange={(e) => setProtocol(e.target.value)}
              className={inputClass}
            >
              <option value="vless">VLESS</option>
              <option value="vmess">VMess</option>
              <option value="trojan">Trojan</option>
              <option value="shadowsocks">Shadowsocks</option>
              <option value="socks">SOCKS</option>
              <option value="http">HTTP</option>
            </select>
          </Field>

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
            name="network"
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
            <Field label="HTTP-маскировка">
              <select
                value={stream.tcpHeaderType}
                onChange={(e) =>
                  setStream({ ...stream, tcpHeaderType: e.target.value as 'none' | 'http' })
                }
                className={inputClass}
              >
                <option value="none">Нет</option>
                <option value="http">HTTP</option>
              </select>
            </Field>
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
            <div className="grid grid-cols-2 gap-4">
              <Field label="Path">
                <input
                  value={stream.xhttpPath}
                  onChange={(e) => setStream({ ...stream, xhttpPath: e.target.value })}
                  className={inputClass}
                />
              </Field>
              <Field label="Mode">
                <select
                  value={stream.xhttpMode}
                  onChange={(e) => setStream({ ...stream, xhttpMode: e.target.value })}
                  className={inputClass}
                >
                  <option value="auto">auto</option>
                  <option value="packet-up">packet-up</option>
                  <option value="stream-up">stream-up</option>
                  <option value="stream-one">stream-one</option>
                </select>
              </Field>
            </div>
          )}
        </Section>

        {/* Безопасность */}
        <Section title="Безопасность">
          <Radio
            name="security"
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
                <Field label="Fingerprint">
                  <select
                    value={stream.tlsFingerprint}
                    onChange={(e) => setStream({ ...stream, tlsFingerprint: e.target.value })}
                    className={inputClass}
                  >
                    <option value="chrome">chrome</option>
                    <option value="firefox">firefox</option>
                    <option value="safari">safari</option>
                    <option value="random">random</option>
                  </select>
                </Field>
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
                    className="bg-neutral-800 hover:bg-neutral-700 text-white text-sm rounded px-3 py-2 whitespace-nowrap disabled:opacity-50"
                  >
                    {generating ? '...' : 'Сгенерировать'}
                  </button>
                </div>
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
                    className="bg-neutral-800 hover:bg-neutral-700 text-white text-sm rounded px-3 py-2 whitespace-nowrap disabled:opacity-50"
                  >
                    + ShortID
                  </button>
                </div>
              </Field>

              <div className="grid grid-cols-2 gap-4">
                <Field label="Fingerprint">
                  <select
                    value={stream.realityFingerprint}
                    onChange={(e) => setStream({ ...stream, realityFingerprint: e.target.value })}
                    className={inputClass}
                  >
                    <option value="chrome">chrome</option>
                    <option value="firefox">firefox</option>
                    <option value="safari">safari</option>
                    <option value="random">random</option>
                  </select>
                </Field>
                <Field label="Flow">
                  <select
                    value={stream.realityFlow}
                    onChange={(e) => setStream({ ...stream, realityFlow: e.target.value })}
                    className={inputClass}
                  >
                    <option value="">(нет)</option>
                    <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                  </select>
                </Field>
              </div>
            </div>
          )}
        </Section>

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

        {/* Settings и Sniffing — оставим как JSON, но в свёрнутом виде */}
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
  'w-full bg-neutral-950 border border-neutral-800 rounded px-3 py-2 text-white outline-none focus:border-neutral-600'

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
  name,
  value,
  onChange,
  options,
}: {
  name: string
  value: string
  onChange: (v: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {options.map((o) => (
        <label
          key={o.value}
          className={
            'text-sm rounded px-3 py-1.5 cursor-pointer transition-colors ' +
            (value === o.value
              ? 'bg-white text-black'
              : 'bg-neutral-800 text-neutral-300 hover:bg-neutral-700')
          }
        >
          <input
            type="radio"
            name={name}
            value={o.value}
            checked={value === o.value}
            onChange={() => onChange(o.value)}
            className="sr-only"
          />
          {o.label}
        </label>
      ))}
    </div>
  )
}
