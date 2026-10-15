import { useEffect, useState } from 'react'
import { Search, ChevronLeft, ChevronRight } from 'lucide-react'
import { getAudit } from '../api/audit'
import { auditLabel } from '../lib/auditLabels'
import { ApiError } from '../api/client'
import { Button } from '../components/ui'
import { TableSkeleton } from '../components/Skeleton'
import type { AuditEntry } from '../api/types'
import { ErrorBanner } from '../components/ErrorBanner'

const PAGE_SIZE = 50

export function Audit() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Debounce: применяем запрос через 300 мс после последнего ввода.
  useEffect(() => {
    const t = setTimeout(() => {
      setDebouncedQuery(query)
      setOffset(0)
    }, 300)
    return () => clearTimeout(t)
  }, [query])

  async function load() {
    setLoading(true)
    setError('')
    try {
      const res = await getAudit(PAGE_SIZE, offset, debouncedQuery)
      setEntries(res.entries ?? [])
      setTotal(res.total ?? 0)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка загрузки')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [offset, debouncedQuery])

  const pages = Math.ceil(total / PAGE_SIZE)
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1

  return (
    <div className="flex-1 p-6 overflow-y-auto">
      {/* Заголовок */}
      <div className="flex items-center justify-between mb-6 gap-4">
        <div>
          <h2 className="text-2xl font-bold text-white">Аудит</h2>
          <p className="text-sm mt-1" style={{ color: 'var(--text-muted)' }}>
            {total} {total === 1 ? 'событие' : total < 5 ? 'события' : 'событий'}
          </p>
        </div>

        {/* Поиск */}
        <div className="relative w-72">
          <Search
            size={16}
            className="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none"
            style={{ color: 'var(--text-dim)' }}
          />
          <input
            type="text"
            placeholder="Поиск по действию, объекту, IP"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="w-full rounded-lg pl-9 pr-3 py-2 text-sm outline-none transition-colors"
            style={{
              background: '#0a0a0a',
              border: '1px solid var(--border)',
              color: 'var(--text)',
            }}
            onFocus={(e) => {
              e.currentTarget.style.borderColor = 'var(--accent)'
            }}
            onBlur={(e) => {
              e.currentTarget.style.borderColor = 'var(--border)'
            }}
          />
        </div>
      </div>

      {error && (
        <div className="mb-4">
          <ErrorBanner
            title="Не удалось загрузить журнал аудита"
            message={error}
            onRetry={load}
          />
        </div>
      )}

      {loading ? (
        <TableSkeleton rows={10} cols={4} />
      ) : entries.length === 0 ? (
        <div className="card p-12 text-center">
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            {debouncedQuery ? 'Ничего не найдено' : 'Событий пока нет'}
          </p>
        </div>
      ) : (
        <>
          <div className="card overflow-hidden">
            <table className="table">
              <thead>
                <tr>
                  <th style={{ width: '180px' }}>Время</th>
                  <th>Действие</th>
                  <th>Объект</th>
                  <th style={{ width: '140px' }}>IP</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => (
                  <tr key={e.id}>
                    <td
                      className="whitespace-nowrap text-xs"
                      style={{ color: 'var(--text-muted)' }}
                    >
                      {new Date(e.ts * 1000).toLocaleString('ru-RU')}
                    </td>
                    <td className="text-white">{e.action_ru || auditLabel(e.action)}</td>
                    <td style={{ color: 'var(--text-muted)' }}>{e.target || '—'}</td>
                    <td
                      className="font-mono text-xs"
                      style={{ color: 'var(--text-dim)' }}
                    >
                      {e.ip}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {pages > 1 && (
            <div className="flex items-center justify-between mt-4 text-sm">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                disabled={offset === 0}
                icon={<ChevronLeft size={14} />}
              >
                Назад
              </Button>
              <span style={{ color: 'var(--text-muted)' }}>
                Страница {currentPage} из {pages}
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setOffset(offset + PAGE_SIZE)}
                disabled={currentPage >= pages}
              >
                Вперёд
                <ChevronRight size={14} />
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
