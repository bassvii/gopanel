import { useEffect, useState } from 'react'
import { Sparkles, Loader2 } from 'lucide-react'
import { listPresets, applyPreset, type Preset } from '../api/routing'
import { ApiError } from '../api/client'
import { toast } from 'sonner'
import { useConfirm } from '../hooks/useConfirm'

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
