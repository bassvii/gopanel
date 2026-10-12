import { useEffect, useState } from 'react'
import { Bell, Check } from 'lucide-react'
import { listNotifications, markAllRead, type Notification } from '../api/notifications'
import { ApiError } from '../api/client'
import { toast } from 'sonner'
import { Dropdown } from './ui'
import { useRouter } from '../lib/router'

export function NotificationsBell() {
  const [items, setItems] = useState<Notification[]>([])
  const [unread, setUnread] = useState(0)
  const [loading, setLoading] = useState(false)
  const router = useRouter()

  async function reload() {
    try {
      const res = await listNotifications(20)
      setItems(res.notifications)
      setUnread(res.unread)
    } catch {
      /* silent */
    }
  }

  useEffect(() => {
    reload()
    // Автообновление раз в 30 секунд.
    const t = setInterval(reload, 30_000)
    return () => clearInterval(t)
  }, [])

  async function handleMarkAll() {
    setLoading(true)
    try {
      const n = await markAllRead()
      setUnread(n)
      setItems((prev) => prev.map((it) => ({ ...it, read_at: it.read_at || Date.now() / 1000 })))
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка')
    } finally {
      setLoading(false)
    }
  }

  function handleOpenAudit() {
    router.navigate({ name: 'audit' })
  }

  return (
    <Dropdown
      align="right"
      width={360}
      trigger={(open) => (
        <div
          className="relative w-9 h-9 rounded-lg flex items-center justify-center transition-colors"
          style={{
            color: open ? 'var(--text)' : 'var(--text-muted)',
            background: open ? '#1a1a1a' : 'transparent',
          }}
          onMouseEnter={(e) => {
            if (!open) {
              e.currentTarget.style.background = '#1a1a1a'
              e.currentTarget.style.color = 'var(--text)'
            }
          }}
          onMouseLeave={(e) => {
            if (!open) {
              e.currentTarget.style.background = 'transparent'
              e.currentTarget.style.color = 'var(--text-muted)'
            }
          }}
        >
          <Bell size={18} />
          {unread > 0 && (
            <div
              className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full flex items-center justify-center text-[10px] font-bold text-white"
              style={{ background: 'var(--danger)' }}
            >
              {unread > 99 ? '99+' : unread}
            </div>
          )}
        </div>
      )}
    >
      {/* Заголовок */}
      <div
        className="px-4 py-3 flex items-center justify-between"
        style={{ borderBottom: '1px solid var(--border)' }}
      >
        <div className="text-sm font-medium text-white">Уведомления</div>
        {unread > 0 && (
          <button
            onClick={handleMarkAll}
            disabled={loading}
            className="text-xs flex items-center gap-1 disabled:opacity-50"
            style={{ color: 'var(--accent)' }}
          >
            <Check size={12} />
            Прочитать все
          </button>
        )}
      </div>

      {/* Список */}
      <div style={{ maxHeight: 400, overflowY: 'auto' }}>
        {items.length === 0 ? (
          <div className="px-4 py-8 text-center text-xs" style={{ color: 'var(--text-muted)' }}>
            Уведомлений нет
          </div>
        ) : (
          items.map((n) => <NotificationItem key={n.id} n={n} />)
        )}
      </div>

      {/* Footer */}
      <div
        className="px-4 py-2 flex items-center justify-center"
        style={{ borderTop: '1px solid var(--border)' }}
      >
        <button
          onClick={handleOpenAudit}
          className="text-xs hover:text-white"
          style={{ color: 'var(--text-muted)' }}
        >
          Открыть аудит →
        </button>
      </div>
    </Dropdown>
  )
}

function NotificationItem({ n }: { n: Notification }) {
  const isUnread = !n.read_at
  const dotColor =
    n.severity === 'error'
      ? 'var(--danger)'
      : n.severity === 'warning'
        ? 'var(--warning)'
        : 'var(--accent)'

  return (
    <div
      className="px-4 py-3 flex gap-3"
      style={{
        borderBottom: '1px solid var(--border)',
        background: isUnread ? 'rgba(59, 130, 246, 0.04)' : 'transparent',
      }}
    >
      <div
        className="w-1.5 h-1.5 rounded-full mt-1.5 shrink-0"
        style={{ background: dotColor }}
      />
      <div className="min-w-0 flex-1">
        <div className="text-sm text-white">{n.title}</div>
        {n.body && (
          <div className="text-xs mt-0.5 break-words" style={{ color: 'var(--text-muted)' }}>
            {n.body}
          </div>
        )}
        <div className="text-xs mt-1" style={{ color: 'var(--text-dim)' }}>
          {new Date(n.created_at * 1000).toLocaleString('ru-RU')}
        </div>
      </div>
    </div>
  )
}
