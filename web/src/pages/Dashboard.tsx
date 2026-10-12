import { useEffect, useState } from 'react'
import { Activity, Cpu, HardDrive, MemoryStick, Server, Clock } from 'lucide-react'
import { getStatsSummary } from '../api/stats'
import { getAudit } from '../api/audit'
import { getSystemStats, type SystemStats } from '../api/system'
import { formatBytes } from '../lib/format'
import { auditLabel } from '../lib/auditLabels'
import { useRouter } from '../lib/router'
import { ApiError } from '../api/client'
import { Sparkline } from '../components/Sparkline'
import { CardSkeleton } from '../components/Skeleton'
import type { AuditEntry, StatsSummary } from '../api/types'

export function Dashboard() {
  const { navigate } = useRouter()
  const [summary, setSummary] = useState<StatsSummary | null>(null)
  const [system, setSystem] = useState<SystemStats | null>(null)
  const [audit, setAudit] = useState<AuditEntry[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([getStatsSummary(), getAudit(5), getSystemStats()])
      .then(([s, a, sys]) => {
        setSummary(s)
        setAudit(a.entries)
        setSystem(sys)
      })
      .catch((err) => {
        setError(err instanceof ApiError ? err.message : 'Ошибка загрузки')
      })

    // Обновляем метрики раз в 5 секунд.
    const t = setInterval(() => {
      getSystemStats().then(setSystem).catch(() => {})
    }, 5000)

    return () => clearInterval(t)
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
            className="text-sm bg-neutral-800 hover:bg-neutral-700 text-white rounded-lg px-3 py-1.5"
          >
            + Пользователь
          </button>
          <button
            onClick={() => navigate({ name: 'inbounds' })}
            className="text-sm bg-neutral-800 hover:bg-neutral-700 text-white rounded-lg px-3 py-1.5"
          >
            + Инбаунд
          </button>
        </div>
      </div>

      {/* Системные метрики */}
      {system === null ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <CardSkeleton />
          <CardSkeleton />
          <CardSkeleton />
          <CardSkeleton />
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <MetricCard
            icon={<Cpu size={16} />}
            title="CPU"
            value={`${system.current.cpu_percent.toFixed(1)}%`}
            subtitle={`${system.current.cpu_cores} ядер`}
            data={system.history.map((p) => p.cpu_percent)}
            color="#3b82f6"
          />
          <MetricCard
            icon={<MemoryStick size={16} />}
            title="RAM"
            value={`${system.current.mem_percent.toFixed(0)}%`}
            subtitle={`${formatBytes(system.current.mem_used_bytes)} / ${formatBytes(system.current.mem_total_bytes)}`}
            data={system.history.map((p) => p.mem_percent)}
            color="#22c55e"
          />
          <MetricCard
            icon={<HardDrive size={16} />}
            title="Диск"
            value={`${system.current.disk_percent.toFixed(0)}%`}
            subtitle={`${formatBytes(system.current.disk_used_bytes)} / ${formatBytes(system.current.disk_total_bytes)}`}
            data={system.history.map((p) => p.disk_percent)}
            color="#eab308"
          />
          <MetricCard
            icon={<Activity size={16} />}
            title="Load Average"
            value={system.current.load_avg_1.toFixed(2)}
            subtitle={`${system.current.load_avg_5.toFixed(2)} / ${system.current.load_avg_15.toFixed(2)}`}
            data={system.history.map((p) => p.load_avg_1)}
            color="#a855f7"
          />
        </div>
      )}

      {/* Строка статуса */}
      {system && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <StatusCard
            icon={<Server size={16} />}
            title="Xray"
            status={system.xray.running ? 'running' : 'stopped'}
            subtitle={system.xray.running ? `v${system.xray.version}` : 'не запущен'}
            extra={
              system.xray.running
                ? `uptime ${formatUptime(system.xray.uptime_seconds)}`
                : undefined
            }
          />
          <StatusCard
            icon={<Clock size={16} />}
            title="Панель"
            status="running"
            subtitle={`v${system.panel.version}`}
            extra={`uptime ${formatUptime(system.panel.uptime_seconds)}`}
          />
          <StatusCard
            icon={<HardDrive size={16} />}
            title="Сервер"
            status="running"
            subtitle={`uptime ${formatUptime(system.current.uptime_seconds)}`}
            extra={`БД ${formatBytes(system.panel.db_size_bytes)}`}
          />
        </div>
      )}

      {/* Сводка */}
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
      <div className="card">
        <div
          className="px-5 py-3 flex items-center justify-between"
          style={{ borderBottom: '1px solid var(--border)' }}
        >
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
          <ul>
            {audit.map((e, i) => (
              <li
                key={e.id}
                className="px-5 py-3 flex items-center justify-between text-sm"
                style={{ borderTop: i === 0 ? 'none' : '1px solid var(--border)' }}
              >
                <div className="flex items-center gap-3">
                  <span className="text-neutral-300">
                    {e.action_ru || auditLabel(e.action)}
                  </span>
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

// --- Карточки ---

function MetricCard({
  icon,
  title,
  value,
  subtitle,
  data,
  color,
}: {
  icon: React.ReactNode
  title: string
  value: string
  subtitle: string
  data?: number[]
  color: string
}) {
  return (
    <div className="card p-5">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2 text-xs text-neutral-500 uppercase tracking-wide">
          {icon}
          <span>{title}</span>
        </div>
      </div>
      <div className="text-2xl font-bold text-white mb-1">{value}</div>
      <div className="text-xs text-neutral-600 mb-3">{subtitle}</div>
      {data && data.length > 1 && (
        <Sparkline data={data} color={color} width={200} height={32} />
      )}
    </div>
  )
}

function StatusCard({
  icon,
  title,
  status,
  subtitle,
  extra,
}: {
  icon: React.ReactNode
  title: string
  status: 'running' | 'stopped'
  subtitle: string
  extra?: string
}) {
  return (
    <div className="card p-4">
      <div className="flex items-center gap-2 mb-2">
        <div
          className="w-2 h-2 rounded-full"
          style={{
            background: status === 'running' ? 'var(--success)' : 'var(--danger)',
          }}
        />
        <div className="flex items-center gap-1.5 text-xs text-neutral-500 uppercase tracking-wide">
          {icon}
          <span>{title}</span>
        </div>
      </div>
      <div className="text-sm text-white">{subtitle}</div>
      {extra && <div className="text-xs text-neutral-600 mt-1">{extra}</div>}
    </div>
  )
}

function StatCard({ title, value, subtitle }: { title: string; value: string; subtitle: string }) {
  return (
    <div className="card p-5">
      <div className="text-xs text-neutral-500 uppercase tracking-wide mb-2">{title}</div>
      <div className="text-2xl font-bold text-white mb-1">{value}</div>
      <div className="text-xs text-neutral-600">{subtitle}</div>
    </div>
  )
}

// --- Утилиты ---

function formatUptime(seconds: number): string {
  if (!seconds || seconds <= 0) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}д ${h}ч`
  if (h > 0) return `${h}ч ${m}м`
  return `${m}м`
}
