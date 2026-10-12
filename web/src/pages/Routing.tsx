import {
  createContext,
  useContext,
  useEffect,
  useState,
  type FormEvent,
} from 'react'
import {
  Loader2,
  Plus,
  Pencil,
  Trash2,
  ArrowUp,
  ArrowDown,
} from 'lucide-react'
import {
  listPresets,
  applyPreset,
  listRoutingRules,
  createRoutingRule,
  updateRoutingRule,
  deleteRoutingRule,
  moveRoutingRule,
  type Preset,
  type RoutingRule,
  type RoutingRuleInput,
} from '../api/routing'
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
import { Button, Badge, Toggle, Select, Tooltip } from '../components/ui'
import { Modal } from '../components/Modal'
import { RefreshCw } from 'lucide-react'
import { getGeoStatus, updateGeo, type GeoStatus } from '../api/geo'
import { formatBytes } from '../lib/format'

// --- Контекст обновления ---

const RefreshContext = createContext<() => void>(() => {})

function useRefresh() {
  return useContext(RefreshContext)
}

// --- Страница ---

export function Routing() {
  const [refreshKey, setRefreshKey] = useState(0)

  const refreshAll = () => setRefreshKey((k) => k + 1)

  return (
    <RefreshContext.Provider value={refreshAll}>
      <div className="flex-1 p-6 overflow-y-auto space-y-6">
        {/* Шапка с пресетами */}
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div>
            <h2 className="text-2xl font-bold text-white">Маршрутизация</h2>
            <p className="text-sm mt-1" style={{ color: 'var(--text-muted)' }}>
              Outbound'ы и правила распределения трафика
            </p>
          </div>

          <PresetsBar />
        </div>

        <OutboundsSection key={`out-${refreshKey}`} />
        <RulesSection key={`rules-${refreshKey}`} />
        <GeoSection />
      </div>
    </RefreshContext.Provider>
  )
}

// --- Пресеты (компактная панель) ---

function PresetsBar() {
  const [presets, setPresets] = useState<Preset[]>([])
  const [loading, setLoading] = useState(true)
  const [applying, setApplying] = useState<string | null>(null)
  const { confirm, dialog } = useConfirm()
  const refreshAll = useRefresh()

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
      message: `Будет добавлено ${p.rule_count} новых правил.\n\n${p.description}\n\nСуществующие правила не удаляются.`,
      confirmLabel: 'Применить',
    })
    if (!ok) return

    setApplying(p.name)
    try {
      const created = await applyPreset(p.name)
      toast.success(`Пресет применён: добавлено ${created} правил`)
      refreshAll()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка применения')
    } finally {
      setApplying(null)
    }
  }

  if (loading) {
    return (
      <div className="text-xs" style={{ color: 'var(--text-muted)' }}>
        Загрузка пресетов...
      </div>
    )
  }

  return (
    <>
      <div className="flex flex-col items-end gap-2">
        <div
          className="text-xs uppercase tracking-wide"
          style={{ color: 'var(--text-dim)' }}
        >
          Быстрая настройка
        </div>
        <div className="flex flex-wrap gap-2 justify-end">
          {presets.map((p) => (
            <Tooltip
              key={p.name}
              content={`${p.description} (${p.rule_count} ${
                p.rule_count === 1 ? 'правило' : p.rule_count < 5 ? 'правила' : 'правил'
              })`}
            >
              <button
                onClick={() => handleApply(p)}
                disabled={applying !== null}
                className="text-xs px-3 py-2 rounded-lg transition-colors flex items-center gap-1.5 disabled:opacity-50"
                style={{
                  background: 'var(--bg-surface)',
                  border: '1px solid var(--border)',
                  color: 'var(--text-muted)',
                }}
                onMouseEnter={(e) => {
                  if (applying === null) {
                    e.currentTarget.style.background = 'var(--bg-surface-hover)'
                    e.currentTarget.style.borderColor = 'var(--border-strong)'
                    e.currentTarget.style.color = 'var(--text)'
                  }
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = 'var(--bg-surface)'
                  e.currentTarget.style.borderColor = 'var(--border)'
                  e.currentTarget.style.color = 'var(--text-muted)'
                }}
              >
                {applying === p.name ? (
                  <Loader2 size={12} className="animate-spin" />
                ) : (
                  <Plus size={12} />
                )}
                {presetLabel(p.name)}
              </button>
            </Tooltip>
          ))}
        </div>
      </div>

      {dialog}
    </>
  )
}

// presetLabel — короткое читаемое имя пресета.
function presetLabel(name: string): string {
  const labels: Record<string, string> = {
    'full-ru-split': 'Полный РФ-сплит',
    'ru-direct': 'РФ — напрямую',
    'ads-block': 'Реклама — блок',
    'private-direct': 'Приватные — напрямую',
    'everything-proxy': 'Всё — через PROXY',
  }
  return labels[name] ?? name
}

// --- Outbound'ы ---

function OutboundsSection() {
  const [outbounds, setOutbounds] = useState<Outbound[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<Outbound | null>(null)
  const { confirm, dialog } = useConfirm()
  const refreshAll = useRefresh()

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
      toast.success('Outbound удалён')
      refreshAll()
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
      toast.success(o.enabled ? 'Outbound отключён' : 'Outbound включён')
      refreshAll()
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
          <Button onClick={() => setCreating(true)} icon={<Plus size={14} />} size="sm">
            Создать
          </Button>
        </div>

        {loading ? (
          <div className="p-5 text-sm" style={{ color: 'var(--text-muted)' }}>
            Загрузка...
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
                      {o.is_builtin && <Badge variant="neutral">системный</Badge>}
                    </div>
                  </td>
                  <td>
                    <Badge variant="neutral">{o.protocol}</Badge>
                  </td>
                  <td>
                    <Toggle enabled={o.enabled} onChange={() => toggleEnabled(o)} />
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
          onSaved={() => {
            setCreating(false)
            refreshAll()
          }}
        />
      )}

      {editing && (
        <OutboundForm
          outbound={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null)
            refreshAll()
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
              className="input"
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
              <span className="text-sm text-neutral-400 block mb-1">Settings (JSON)</span>
              <textarea
                value={settings}
                onChange={(e) => setSettings(e.target.value)}
                rows={8}
                placeholder='{"vnext":[{"address":"1.2.3.4","port":443,"users":[{"id":"uuid","encryption":"none"}]}]}'
                className="input font-mono text-xs"
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
                className="input font-mono text-xs"
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

// --- Правила ---

function RulesSection() {
  const [rules, setRules] = useState<RoutingRule[]>([])
  const [outbounds, setOutbounds] = useState<Outbound[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<RoutingRule | null>(null)
  const { confirm, dialog } = useConfirm()
  const refreshAll = useRefresh()

  async function reload() {
    setLoading(true)
    try {
      const [r, o] = await Promise.all([listRoutingRules(), listOutbounds()])
      setRules(r)
      setOutbounds(o)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка загрузки')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [])

  async function handleDelete(r: RoutingRule) {
    const ok = await confirm({
      title: 'Удалить правило?',
      message: `Правило «${r.rule_tag || 'без имени'}» будет удалено.`,
      confirmLabel: 'Удалить',
      danger: true,
    })
    if (!ok) return
    try {
      await deleteRoutingRule(r.id)
      toast.success('Правило удалено')
      refreshAll()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка удаления')
    }
  }

  async function handleMove(r: RoutingRule, dir: 'up' | 'down') {
    try {
      await moveRoutingRule(r.id, dir)
      refreshAll()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка перемещения')
    }
  }

  async function toggleEnabled(r: RoutingRule) {
    try {
      await updateRoutingRule(r.id, {
        rule_tag: r.rule_tag,
        domains_json: r.domains_json,
        ips_json: r.ips_json,
        ports_json: r.ports_json,
        source_ips_json: r.source_ips_json,
        network: r.network,
        inbound_tags_json: r.inbound_tags_json,
        protocol_json: r.protocol_json,
        outbound_tag: r.outbound_tag,
        enabled: !r.enabled,
      })
      toast.success(r.enabled ? 'Правило отключено' : 'Правило включено')
      refreshAll()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка')
    }
  }

  const outboundTags = outbounds.map((o) => o.tag)

  return (
    <>
      <div className="card">
        <div
          className="px-5 py-4 flex items-center justify-between"
          style={{ borderBottom: '1px solid var(--border)' }}
        >
          <div>
            <div className="text-sm font-medium text-white">Правила маршрутизации</div>
            <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
              Применяются сверху вниз — первое совпадение выигрывает
            </div>
          </div>
          <Button onClick={() => setCreating(true)} icon={<Plus size={14} />} size="sm">
            Создать
          </Button>
        </div>

        {loading ? (
          <div className="p-5 text-sm" style={{ color: 'var(--text-muted)' }}>
            Загрузка...
          </div>
        ) : rules.length === 0 ? (
          <div className="p-8 text-center">
            <p className="text-sm mb-2" style={{ color: 'var(--text-muted)' }}>
              Правил пока нет
            </p>
            <p className="text-xs" style={{ color: 'var(--text-dim)' }}>
              Примените пресет выше или создайте правило вручную
            </p>
          </div>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th style={{ width: '60px' }}>#</th>
                <th>Правило</th>
                <th>Условие</th>
                <th style={{ width: '120px' }}>Действие</th>
                <th style={{ width: '80px' }}>Статус</th>
                <th style={{ textAlign: 'right', width: '200px' }}>Действия</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((r, i) => (
                <tr key={r.id}>
                  <td style={{ color: 'var(--text-dim)' }}>{r.position}</td>
                  <td>
                    <span className="text-white">{r.rule_tag || '—'}</span>
                  </td>
                  <td style={{ color: 'var(--text-muted)' }}>
                    <ConditionCell rule={r} />
                  </td>
                  <td>
                    <OutboundBadge tag={r.outbound_tag} />
                  </td>
                  <td>
                    <Toggle enabled={r.enabled} onChange={() => toggleEnabled(r)} />
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleMove(r, 'up')}
                        disabled={i === 0}
                        title="Выше"
                      >
                        <ArrowUp size={14} />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleMove(r, 'down')}
                        disabled={i === rules.length - 1}
                        title="Ниже"
                      >
                        <ArrowDown size={14} />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditing(r)}
                        icon={<Pencil size={14} />}
                      >
                        Изменить
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDelete(r)}
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
        )}
      </div>

      {creating && (
        <RuleForm
          outboundTags={outboundTags}
          onClose={() => setCreating(false)}
          onSaved={() => {
            setCreating(false)
            refreshAll()
          }}
        />
      )}

      {editing && (
        <RuleForm
          rule={editing}
          outboundTags={outboundTags}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null)
            refreshAll()
          }}
        />
      )}

      {dialog}
    </>
  )
}

// --- Отображение условия ---

function ConditionCell({ rule }: { rule: RoutingRule }) {
  const parts: string[] = []

  const domains = parseArray(rule.domains_json)
  const ips = parseArray(rule.ips_json)
  const ports = parseArray(rule.ports_json)
  const network = rule.network

  if (domains.length > 0) {
    parts.push(`domain: ${truncate(domains, 2)}`)
  }
  if (ips.length > 0) {
    parts.push(`ip: ${truncate(ips, 2)}`)
  }
  if (ports.length > 0) {
    parts.push(`port: ${truncate(ports, 3)}`)
  }
  if (network) {
    parts.push(`network: ${network}`)
  }

  if (parts.length === 0) {
    return <span style={{ color: 'var(--text-dim)' }}>всё</span>
  }

  return (
    <div className="text-xs space-y-0.5 font-mono">
      {parts.map((p, i) => (
        <div key={i}>{p}</div>
      ))}
    </div>
  )
}

function OutboundBadge({ tag }: { tag: string }) {
  let variant: 'success' | 'danger' | 'neutral' = 'neutral'
  if (tag === 'DIRECT') variant = 'success'
  else if (tag === 'BLOCK') variant = 'danger'

  return <Badge variant={variant}>{tag}</Badge>
}

function parseArray(json: string): string[] {
  if (!json || json === '[]') return []
  try {
    const v = JSON.parse(json)
    return Array.isArray(v) ? v.filter((x) => typeof x === 'string') : []
  } catch {
    return []
  }
}

function truncate(arr: string[], n: number): string {
  if (arr.length <= n) return arr.join(', ')
  return arr.slice(0, n).join(', ') + ` +${arr.length - n}`
}

// --- Форма правила ---

interface RuleFormProps {
  rule?: RoutingRule
  outboundTags: string[]
  onClose: () => void
  onSaved: () => void
}

function RuleForm({ rule, outboundTags, onClose, onSaved }: RuleFormProps) {
  const isEdit = !!rule

  const [ruleTag, setRuleTag] = useState(rule?.rule_tag ?? '')
  const [domains, setDomains] = useState(parseArray(rule?.domains_json ?? '').join(', '))
  const [ips, setIPs] = useState(parseArray(rule?.ips_json ?? '').join(', '))
  const [ports, setPorts] = useState(parseArray(rule?.ports_json ?? '').join(', '))
  const [network, setNetwork] = useState(rule?.network ?? '')
  const [outboundTag, setOutboundTag] = useState(rule?.outbound_tag ?? 'DIRECT')
  const [enabled, setEnabled] = useState(rule?.enabled ?? true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSaving(true)

    const input: RoutingRuleInput = {
      rule_tag: ruleTag,
      domains_json: JSON.stringify(splitList(domains)),
      ips_json: JSON.stringify(splitList(ips)),
      ports_json: JSON.stringify(splitList(ports)),
      source_ips_json: '[]',
      network,
      inbound_tags_json: '[]',
      protocol_json: '[]',
      outbound_tag: outboundTag,
      enabled,
    }

    try {
      if (isEdit && rule) {
        await updateRoutingRule(rule.id, input)
        toast.success('Правило обновлено')
      } else {
        await createRoutingRule(input)
        toast.success('Правило создано')
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
      title={isEdit ? `Изменить правило` : 'Новое правило'}
      onClose={onClose}
      wide
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <label className="block">
          <span className="text-sm text-neutral-400 block mb-1">Название правила</span>
          <input
            value={ruleTag}
            onChange={(e) => setRuleTag(e.target.value)}
            placeholder="РФ — напрямую"
            className="input"
          />
        </label>

        <label className="block">
          <span className="text-sm text-neutral-400 block mb-1">
            Домены (через запятую)
          </span>
          <textarea
            value={domains}
            onChange={(e) => setDomains(e.target.value)}
            rows={3}
            placeholder="geosite:category-ru, geosite:private, example.com"
            className="input font-mono text-xs"
          />
          <div className="text-xs mt-1" style={{ color: 'var(--text-dim)' }}>
            <CategoriesHint type="domain" />
          </div>
        </label>

        <label className="block">
          <span className="text-sm text-neutral-400 block mb-1">
            IP-адреса (через запятую)
          </span>
          <textarea
            value={ips}
            onChange={(e) => setIPs(e.target.value)}
            rows={2}
            placeholder="geoip:ru, geoip:private"
            className="input font-mono text-xs"
          />
          <div className="text-xs mt-1" style={{ color: 'var(--text-dim)' }}>
            <CategoriesHint type="ip" />
          </div>
        </label>

        <label className="block">
          <span className="text-sm text-neutral-400 block mb-1">
            Порты (через запятую)
          </span>
          <input
            value={ports}
            onChange={(e) => setPorts(e.target.value)}
            placeholder="80, 443, 1000-2000"
            className="input font-mono text-xs"
          />
        </label>

        <div>
          <span className="text-sm text-neutral-400 block mb-1">Network</span>
          <Select
            value={network}
            onChange={setNetwork}
            options={[
              { value: '', label: 'Любой' },
              { value: 'tcp', label: 'TCP' },
              { value: 'udp', label: 'UDP' },
              { value: 'tcp,udp', label: 'TCP и UDP' },
            ]}
          />
        </div>

        <div>
          <span className="text-sm text-neutral-400 block mb-1">Действие</span>
          <Select
            value={outboundTag}
            onChange={setOutboundTag}
            options={outboundTags.map((t) => ({ value: t, label: t }))}
          />
        </div>

        <label className="flex items-center gap-2 text-sm text-neutral-300">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          Включено
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

// --- Подсказки по категориям ---

function CategoriesHint({ type }: { type: 'domain' | 'ip' }) {
  const domains = [
    { group: 'Россия', items: ['geosite:category-ru'] },
    { group: 'Реклама', items: ['geosite:category-ads-all'] },
    { group: 'Приватные', items: ['geosite:private'] },
    { group: 'Сервисы', items: ['geosite:google', 'geosite:youtube', 'geosite:telegram', 'geosite:discord'] },
  ]
  const ips = [
    { group: 'Россия', items: ['geoip:ru'] },
    { group: 'Приватные', items: ['geoip:private'] },
    { group: 'Китай', items: ['geoip:cn'] },
  ]

  const groups = type === 'domain' ? domains : ips

  return (
    <details>
      <summary className="cursor-pointer hover:text-white">
        Показать популярные категории
      </summary>
      <div className="mt-2 space-y-1">
        {groups.map((g) => (
          <div key={g.group}>
            <span className="font-medium" style={{ color: 'var(--text-muted)' }}>
              {g.group}:
            </span>{' '}
            {g.items.map((item, i) => (
              <span key={item}>
                <code style={{ color: 'var(--accent)' }}>{item}</code>
                {i < g.items.length - 1 ? ', ' : ''}
              </span>
            ))}
          </div>
        ))}
      </div>
    </details>
  )
}

// --- Утилиты ---

function splitList(s: string): string[] {
  return s
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
}

// --- GeoIP/GeoSite ---

function GeoSection() {
  const [status, setStatus] = useState<GeoStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [updating, setUpdating] = useState(false)

  async function reload() {
    setLoading(true)
    try {
      setStatus(await getGeoStatus())
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка загрузки')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [])

  async function handleUpdate() {
    setUpdating(true)
    try {
      const res = await updateGeo()
      setStatus(res.status)
      toast.success(`Обновлено ${res.updated} файлов`)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка обновления')
    } finally {
      setUpdating(false)
    }
  }

  return (
    <div className="card">
      <div
        className="px-5 py-4 flex items-center justify-between"
        style={{ borderBottom: '1px solid var(--border)' }}
      >
        <div>
          <div className="text-sm font-medium text-white">GeoIP / GeoSite</div>
          <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
            Библиотеки категорий для маршрутизации (Россия, реклама, сервисы)
          </div>
        </div>
        <Button
          onClick={handleUpdate}
          disabled={updating}
          icon={
            updating ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              <RefreshCw size={14} />
            )
          }
          size="sm"
        >
          {updating ? 'Обновление...' : 'Обновить'}
        </Button>
      </div>

      <div className="p-5">
        {loading ? (
          <div className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Загрузка...
          </div>
        ) : status ? (
          <div className="grid grid-cols-2 gap-4">
            <GeoFileCard file={status.geosite} />
            <GeoFileCard file={status.geoip} />
          </div>
        ) : null}
      </div>
    </div>
  )
}

function GeoFileCard({ file }: { file: GeoStatus['geosite'] }) {
  return (
    <div
      className="rounded-lg p-3"
      style={{ background: '#0a0a0a', border: '1px solid var(--border)' }}
    >
      <div className="flex items-center gap-2 mb-2">
        <div
          className="w-2 h-2 rounded-full"
          style={{ background: file.exists ? 'var(--success)' : 'var(--danger)' }}
        />
        <span className="text-sm font-mono text-white">{file.name}</span>
      </div>
      {file.exists ? (
        <>
          <div className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Размер: {formatBytes(file.size_bytes)}
          </div>
          <div className="text-xs mt-0.5" style={{ color: 'var(--text-dim)' }}>
            Обновлён: {new Date(file.modified_at * 1000).toLocaleString('ru-RU')}
          </div>
        </>
      ) : (
        <div className="text-xs" style={{ color: 'var(--danger)' }}>
          Файл не найден — нажмите «Обновить»
        </div>
      )}
    </div>
  )
}
