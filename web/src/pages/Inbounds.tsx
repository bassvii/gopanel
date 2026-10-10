import { useEffect, useState, type FormEvent } from 'react'
import { Modal } from '../components/Modal'
import { listInbounds, createInbound, updateInbound, deleteInbound, type InboundInput } from '../api/inbounds'
import { ApiError } from '../api/client'
import type { Inbound } from '../api/types'

// Пресеты протоколов — предзаполненные формы.
interface Preset {
  name: string
  protocol: string
  defaultPort: number
  settings: string
  stream: string
  sniffing: string
}

const presets: Preset[] = [
  {
    name: 'VLESS',
    protocol: 'vless',
    defaultPort: 443,
    settings: JSON.stringify({ decryption: 'none' }, null, 2),
    stream: JSON.stringify({ network: 'tcp', security: 'none' }, null, 2),
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'VLESS + REALITY',
    protocol: 'vless',
    defaultPort: 443,
    settings: JSON.stringify({ decryption: 'none' }, null, 2),
    stream: JSON.stringify(
      {
        network: 'tcp',
        security: 'reality',
        realitySettings: {
          dest: 'www.google.com:443',
          serverNames: ['www.google.com'],
          privateKey: '',
          shortIds: [''],
        },
      },
      null,
      2,
    ),
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'VLESS + WS',
    protocol: 'vless',
    defaultPort: 443,
    settings: JSON.stringify({ decryption: 'none' }, null, 2),
    stream: JSON.stringify(
      {
        network: 'ws',
        security: 'none',
        wsSettings: { path: '/ws' },
      },
      null,
      2,
    ),
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'VMess',
    protocol: 'vmess',
    defaultPort: 443,
    settings: JSON.stringify({}, null, 2),
    stream: JSON.stringify({ network: 'tcp', security: 'none' }, null, 2),
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'Trojan',
    protocol: 'trojan',
    defaultPort: 443,
    settings: JSON.stringify({}, null, 2),
    stream: JSON.stringify(
      {
        network: 'tcp',
        security: 'tls',
        tlsSettings: { serverName: 'example.com' },
      },
      null,
      2,
    ),
    sniffing: JSON.stringify({ enabled: false }, null, 2),
  },
  {
    name: 'Shadowsocks',
    protocol: 'shadowsocks',
    defaultPort: 8388,
    settings: JSON.stringify({ method: 'aes-256-gcm' }, null, 2),
    stream: JSON.stringify({ network: 'tcp', security: 'none' }, null, 2),
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

      {/* Пресеты */}
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

// --- Форма инбаунда ---

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
  const [port, setPort] = useState(String(inbound?.port ?? preset?.defaultPort ?? 443))
  const [listen, setListen] = useState(inbound?.listen ?? '0.0.0.0')
  const [settings, setSettings] = useState(inbound?.settings_json ?? preset?.settings ?? '{}')
  const [stream, setStream] = useState(inbound?.stream_json ?? preset?.stream ?? '{}')
  const [sniffing, setSniffing] = useState(inbound?.sniffing_json ?? preset?.sniffing ?? '{}')
  const [enabled, setEnabled] = useState(inbound?.enabled ?? true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

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
      stream_json: stream,
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
      <form onSubmit={handleSubmit} className="space-y-4">
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

        <Field label="Settings (JSON)">
          <textarea
            value={settings}
            onChange={(e) => setSettings(e.target.value)}
            rows={6}
            className={inputClass + ' font-mono text-xs'}
          />
        </Field>

        <Field label="Stream Settings (JSON)">
          <textarea
            value={stream}
            onChange={(e) => setStream(e.target.value)}
            rows={8}
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

const inputClass =
  'w-full bg-neutral-950 border border-neutral-800 rounded px-3 py-2 text-white outline-none focus:border-neutral-600'

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-sm text-neutral-400 block mb-1">{label}</span>
      {children}
    </label>
  )
}
