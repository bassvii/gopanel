import { useEffect, useState } from 'react'
import { getAudit } from '../api/audit'
import { ApiError } from '../api/client'
import type { AuditEntry } from '../api/types'
import { capitalize } from '../lib/format'

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

  useEffect(() => {
    setLoading(true)
    setError('')
    getAudit(PAGE_SIZE, offset, debouncedQuery)
      .then((res) => {
        setEntries(res.entries)
        setTotal(res.total)
      })
      .catch((err) => {
        setError(err instanceof ApiError ? err.message : 'Ошибка загрузки')
      })
      .finally(() => setLoading(false))
  }, [offset, debouncedQuery])

  const pages = Math.ceil(total / PAGE_SIZE)
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1

  return (
    <div className="flex-1 p-6 overflow-y-auto">
      <div className="flex items-center justify-between mb-6 gap-4">
        <div>
          <h2 className="text-2xl font-bold text-white">Аудит</h2>
          <p className="text-sm text-neutral-500">{total} событий</p>
        </div>
        <input
          type="text"
          placeholder="Поиск по действию, объекту, IP"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="bg-neutral-950 border border-neutral-800 rounded px-3 py-2 text-white text-sm outline-none focus:border-neutral-600 w-72"
        />
      </div>

      {error && (
        <div className="mb-4 text-sm text-red-400 bg-red-950/30 border border-red-900 rounded px-3 py-2">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-neutral-500">Загрузка...</div>
      ) : entries.length === 0 ? (
        <div className="text-neutral-500 text-center py-16">Событий не найдено</div>
      ) : (
        <>
          <div className="border border-neutral-800 rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-neutral-950 text-neutral-400">
                <tr>
                  <th className="text-left px-4 py-3 font-medium">Время</th>
                  <th className="text-left px-4 py-3 font-medium">Действие</th>
                  <th className="text-left px-4 py-3 font-medium">Объект</th>
                  <th className="text-left px-4 py-3 font-medium">IP</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => (
                  <tr key={e.id} className="border-t border-neutral-800">
                    <td className="px-4 py-2 text-neutral-400 whitespace-nowrap">
                      {new Date(e.ts * 1000).toLocaleString('ru-RU')}
                    </td>
                    <td className="px-4 py-2 text-white">{capitalize(e.action_ru || e.action)}</td>
                    <td className="px-4 py-2 text-neutral-400">{e.target || '—'}</td>
                    <td className="px-4 py-2 text-neutral-500 font-mono text-xs">{e.ip}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {pages > 1 && (
            <div className="flex items-center justify-between mt-4 text-sm">
              <button
                disabled={offset === 0}
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                className="text-neutral-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed"
              >
                ← Назад
              </button>
              <span className="text-neutral-500">
                Страница {currentPage} из {pages}
              </span>
              <button
                disabled={currentPage >= pages}
                onClick={() => setOffset(offset + PAGE_SIZE)}
                className="text-neutral-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed"
              >
                Вперёд →
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
