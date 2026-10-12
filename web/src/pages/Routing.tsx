import { useEffect, useState, type FormEvent } from 'react'
import { Sparkles, Loader2, Plus, Pencil, Trash2 } from 'lucide-react'
import { listPresets, applyPreset, type Preset } from '../api/routing'
import {
  listOutbounds,
  createOutbound,
  updateOutbound,
  deleteOutbound,
  type Outbound,
  type OutboundInput,
} from '../api/outbounds'
import { ApiError } from '../api/client'
import { toast } from 'sonner'
import { useConfirm } from '../hooks/useConfirm'
import { Button, Badge, Toggle, Select } from '../components/ui'
import { Modal } from '../components/Modal'

export function Routing() {
  return (
    <div className="flex-1 p-6 overflow-y-auto space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-white">Маршрутизация</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-muted)' }}>
          Outbound'ы и правила распределения трафика
        </p>
      </div>

      <PresetsSection />
      <OutboundsSection />
    </div>
  )
}

// --- Пресеты ---

function PresetsSection() {
  const [presets, setPresets] = useState<Preset[]>([])
  const [loading, setLoading] = useState(true)
  const [applying, setApplying] = useState<string | null>(null)
  const { confirm, dialog } = useConfirm()

  async function reload() {
    setLoading(true)
    try {
      setPresets(await listPresets())
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка загрузки пресетов')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [])

  async function handleApply(p: Preset) {
    const ok = await confirm({
      title: 'Применить пресет?',
      message: `Будет добавлено ${p.rule_count} новых правил: «${p.description}». Существующие правила не удаляются.`,
      confirmLabel: 'Применить',
    })
    if (!ok) return

    setApplying(p.name)
    try {
      const created = await applyPreset(p.name)
      toast.success(`Пресет применён: добавлено ${created} правил`)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка применения')
    } finally {
      setApplying(null)
    }
  }

  return (
    <div className="card">
      <div
        className="px-5 py-4 flex items-center gap-3"
        style={{ borderBottom: '1px solid var(--border)' }}
      >
        <div
          className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
          style={{ background: 'var(--accent-bg)', color: 'var(--accent)' }}
        >
          <Sparkles size={16} />
        </div>
        <div>
          <div className="text-sm font-medium text-white">Быстрая настройка</div>
          <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
            Готовые наборы правил — один клик
          </div>
        </div>
      </div>

      <div className="p-5">
        {loading ? (
          <div className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Загрузка...
          </div>
        ) : presets.length === 0 ? (
          <div className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Пресетов нет
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {presets.map((p) => (
              <button
                key={p.name}
                onClick={() => handleApply(p)}
                disabled={applying !== null}
                className="text-left p-4 rounded-lg transition-colors disabled:opacity-50"
                style={{
                  background: '#0a0a0a',
                  border: '1px solid var(--border)',
                }}
                onMouseEnter={(e) => {
                  if (applying === null) {
                    e.currentTarget.style.background = 'var(--bg-surface-hover)'
                    e.currentTarget.style.borderColor = 'var(--border-strong)'
                  }
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = '#0a0a0a'
                  e.currentTarget.style.borderColor = 'var(--border)'
                }}
              >
                <div className="flex items-start justify-between gap-2 mb-2">
                  <div className="text-sm font-medium text-white">{p.name}</div>
                  {applying === p.name && (
                    <Loader2 size={14} className="animate-spin" style={{ color: 'var(--accent)' }} />
                  )}
                </div>
                <div className="text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
                  {p.description}
                </div>
                <div className="text-xs" style={{ color: 'var(--text-dim)' }}>
                  {p.rule_count} {p.rule_count === 1 ? 'правило' : p.rule_count < 5 ? 'правила' : 'правил'}
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      {dialog}
    </div>
  )
}

// --- Outbound'ы ---

function OutboundsSection() {
  const [outbounds, setOutbounds] = useState<Outbound[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<Outbound | null>(null)

  const { confirm, dialog } = useConfirm()

  async function reload() {
    setLoading(true)
    try {
      setOutbounds(await listOutbounds())
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка загрузки')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [])

  async function handleDelete(o: Outbound) {
    const ok = await confirm({
      title: 'Удалить outbound?',
      message: `Outbound «${o.tag}» будет удалён. Правила, использующие его, перестанут работать.`,
      confirmLabel: 'Удалить',
      danger: true,
    })
    if (!ok) return
    try {
      await deleteOutbound(o.id)
      await reload()
      toast.success('Outbound удалён')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка удаления')
    }
  }

  async function toggleEnabled(o: Outbound) {
    try {
      await updateOutbound(o.id, {
        tag: o.tag,
        protocol: o.protocol,
        settings_json: o.settings_json,
        stream_json: o.stream_json,
        enabled: !o.enabled,
      })
      await reload()
      toast.success(o.enabled ? 'Outbound отключён' : 'Outbound включён')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка')
    }
  }

  return (
    <>
      <div className="card">
        <div
          className="px-5 py-4 flex items-center justify-between"
          style={{ borderBottom: '1px solid var(--border)' }}
        >
          <div>
            <div className="text-sm font-medium text-white">Outbound'ы</div>
            <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
              Точки выхода: напрямую, блокировка, через другой сервер
            </div>
          </div>
          <Button
            onClick={() => setCreating(true)}
            icon={<Plus size={14} />}
            size="sm"
          >
            Создать
          </Button>
        </div>

        {loading ? (
          <div className="p-5 text-sm" style={{ color: 'var(--text-muted)' }}>
            Загрузка...
          </div>
        ) : outbounds.length === 0 ? (
          <div className="p-5 text-sm" style={{ color: 'var(--text-muted)' }}>
            Outbound'ов нет
          </div>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Тег</th>
                <th>Протокол</th>
                <th>Статус</th>
                <th style={{ textAlign: 'right' }}>Действия</th>
              </tr>
            </thead>
            <tbody>
              {outbounds.map((o) => (
                <tr key={o.id}>
                  <td>
                    <div className="flex items-center gap-2">
                      <span className="text-white font-medium">{o.tag}</span>
                      {o.is_builtin && (
                        <Badge variant="neutral">системный</Badge>
                      )}
                    </div>
                  </td>
                  <td>
                    <Badge variant="neutral">{o.protocol}</Badge>
                  </td>
                  <td>
                    <Toggle
                      enabled={o.enabled}
                      onChange={() => toggleEnabled(o)}
                    />
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditing(o)}
                        icon={<Pencil size={14} />}
                      >
                        Изменить
                      </Button>
                      {!o.is_builtin && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDelete(o)}
                          icon={<Trash2 size={14} />}
                        >
                          Удалить
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {creating && (
        <OutboundForm
          onClose={() => setCreating(false)}
          onSaved={async () => {
            setCreating(false)
            await reload()
          }}
        />
      )}

      {editing && (
        <OutboundForm
          outbound={editing}
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

// --- Форма outbound ---

interface OutboundFormProps {
  outbound?: Outbound
  onClose: () => void
  onSaved: () => void
}

function OutboundForm({ outbound, onClose, onSaved }: OutboundFormProps) {
  const isEdit = !!outbound
  const isBuiltin = outbound?.is_builtin ?? false

  const [tag, setTag] = useState(outbound?.tag ?? '')
  const [protocol, setProtocol] = useState(outbound?.protocol ?? 'vless')
  const [settings, setSettings] = useState(outbound?.settings_json ?? '{}')
  const [stream, setStream] = useState(outbound?.stream_json ?? '{}')
  const [enabled, setEnabled] = useState(outbound?.enabled ?? true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSaving(true)

    const input: OutboundInput = {
      tag,
      protocol,
      settings_json: settings,
      stream_json: stream,
      enabled,
    }

    try {
      if (isEdit && outbound) {
        await updateOutbound(outbound.id, input)
        toast.success('Outbound обновлён')
      } else {
        await createOutbound(input)
        toast.success('Outbound создан')
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
      title={isEdit ? `Изменить «${outbound?.tag}»` : 'Новый outbound'}
      onClose={onClose}
      wide
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="text-sm text-neutral-400 block mb-1">Тег</span>
            <input
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              required
              disabled={isBuiltin}
              placeholder="PROXY"
              className={inputClass}
            />
          </label>

          <div>
            <span className="text-sm text-neutral-400 block mb-1">Протокол</span>
            <Select
              value={protocol}
              onChange={setProtocol}
              disabled={isBuiltin}
              options={[
                { value: 'vless', label: 'VLESS' },
                { value: 'vmess', label: 'VMess' },
                { value: 'trojan', label: 'Trojan' },
                { value: 'shadowsocks', label: 'Shadowsocks' },
                { value: 'socks', label: 'SOCKS' },
                { value: 'http', label: 'HTTP' },
                { value: 'freedom', label: 'Freedom (напрямую)' },
                { value: 'blackhole', label: 'Blackhole (блок)' },
              ]}
            />
          </div>
        </div>

        {!isBuiltin && (
          <>
            <label className="block">
              <span className="text-sm text-neutral-400 block mb-1">
                Settings (JSON)
              </span>
              <textarea
                value={settings}
                onChange={(e) => setSettings(e.target.value)}
                rows={8}
                placeholder='{"vnext":[{"address":"1.2.3.4","port":443,"users":[{"id":"uuid","encryption":"none"}]}]}'
                className={inputClass + ' font-mono text-xs'}
              />
            </label>

            <label className="block">
              <span className="text-sm text-neutral-400 block mb-1">
                Stream Settings (JSON)
              </span>
              <textarea
                value={stream}
                onChange={(e) => setStream(e.target.value)}
                rows={6}
                placeholder='{"network":"tcp","security":"tls"}'
                className={inputClass + ' font-mono text-xs'}
              />
            </label>
          </>
        )}

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
          <Button variant="ghost" type="button" onClick={onClose}>
            Отмена
          </Button>
          <Button type="submit" disabled={saving}>
            {saving ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}

const inputClass =
  'input'
