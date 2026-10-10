import { useEffect, useState } from 'react'
import { getStatsSummary } from '../api/stats'
import { getAudit } from '../api/audit'
import { formatBytes } from '../lib/format'
import { useRouter } from '../lib/router'
import { ApiError } from '../api/client'
import type { AuditEntry, StatsSummary } from '../api/types'
import { auditLabel } from '../lib/auditLabels'

export function Dashboard() {
  const { navigate } = useRouter()
  const [summary, setSummary] = useState<StatsSummary | null>(null)
  const [audit, setAudit] = useState<AuditEntry[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([getStatsSummary(), getAudit(5)])
      .then(([s, a]) => {
        setSummary(s)
        setAudit(a.entries)
      })
      .catch((err) => {
        setError(err instanceof ApiError ? err.message : 'Ошибка загрузки')
      })
  }, [])

  if (error) {
    return (
      <div className="flex-1 p-6">
        <div className="text-sm text-red-400 bg-red-950/30 border border-red-900 rounded px-3 py-2">
          {error}
        </div>
      </div>
    )
  }

  return (
    <div className="flex-1 p-6 overflow-y-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-white">Панель</h2>
          <p className="text-sm text-neutral-500">Обзор состояния сервера</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => navigate({ name: 'users' })}
            className="text-sm bg-neutral-800 hover:bg-neutral-700 text-white rounded px-3 py-1.5"
          >
            + Пользователь
          </button>
          <button
            onClick={() => navigate({ name: 'inbounds' })}
            className="text-sm bg-neutral-800 hover:bg-neutral-700 text-white rounded px-3 py-1.5"
          >
            + Инбаунд
          </button>
        </div>
      </div>

      {/* Карточки */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Пользователи"
          value={summary ? `${summary.users_enabled} / ${summary.users_total}` : '—'}
          subtitle="активных / всего"
        />
        <StatCard
          title="Инбаунды"
          value={summary ? `${summary.inbounds_enabled} / ${summary.inbounds_total}` : '—'}
          subtitle="включённых / всего"
        />
        <StatCard
          title="Использовано"
          value={summary ? formatBytes(summary.traffic_used_bytes) : '—'}
          subtitle="суммарный трафик"
        />
        <StatCard
          title="Общая квота"
          value={summary ? formatBytes(summary.traffic_total_bytes) : '—'}
          subtitle="лимиты всех пользователей"
        />
      </div>

      {/* Последние события */}
      <div className="bg-neutral-950 border border-neutral-800 rounded-lg">
        <div className="px-5 py-3 border-b border-neutral-800 flex items-center justify-between">
          <h3 className="text-sm font-medium text-white">Последние события</h3>
          <button
            onClick={() => navigate({ name: 'audit' })}
            className="text-xs text-neutral-400 hover:text-white"
          >
            Все →
          </button>
        </div>
        {audit.length === 0 ? (
          <div className="px-5 py-8 text-center text-sm text-neutral-500">
            Событий пока нет
          </div>
        ) : (
          <ul className="divide-y divide-neutral-800">
            {audit.map((e) => (
              <li key={e.id} className="px-5 py-3 flex items-center justify-between text-sm">
                <div className="flex items-center gap-3">
                  <span className="text-neutral-300">{auditLabel(e.action)}</span>
                  {e.target && <span className="text-neutral-500">· {e.target}</span>}
                </div>
                <div className="text-xs text-neutral-600">
                  {new Date(e.ts * 1000).toLocaleString('ru-RU')}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function StatCard({ title, value, subtitle }: { title: string; value: string; subtitle: string }) {
  return (
    <div className="bg-neutral-950 border border-neutral-800 rounded-lg p-5">
      <div className="text-xs text-neutral-500 uppercase tracking-wide mb-2">{title}</div>
      <div className="text-2xl font-bold text-white mb-1">{value}</div>
      <div className="text-xs text-neutral-600">{subtitle}</div>
    </div>
  )
}
