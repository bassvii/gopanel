import { Bell, LogOut, User as UserIcon } from 'lucide-react'
import { logout } from '../api/auth'

interface Props {
  title: string
  onLogout: () => void
}

export function Header({ title, onLogout }: Props) {
  async function handleLogout() {
    try {
      await logout()
    } finally {
      onLogout()
    }
  }

  return (
    <header
      className="h-16 flex items-center justify-between px-6 gap-4"
      style={{
        background: 'var(--bg-surface)',
        borderBottom: '1px solid var(--border)',
      }}
    >
      {/* Заголовок */}
      <h1 className="text-lg font-medium text-white">{title}</h1>

      {/* Правый блок */}
      <div className="flex items-center gap-2">
        {/* Индикатор сервера */}
        <div
          className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs"
          style={{ background: '#111111' }}
        >
          <div
            className="w-2 h-2 rounded-full"
            style={{ background: 'var(--success)' }}
          />
          <span style={{ color: 'var(--text-muted)' }}>Сервер онлайн</span>
        </div>

        {/* Уведомления */}
        <button
          className="w-9 h-9 rounded-lg flex items-center justify-center transition-colors"
          style={{ color: 'var(--text-muted)' }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = '#1a1a1a'
            e.currentTarget.style.color = 'var(--text)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = 'transparent'
            e.currentTarget.style.color = 'var(--text-muted)'
          }}
        >
          <Bell size={18} />
        </button>

        {/* Профиль */}
        <div
          className="flex items-center gap-3 pl-2 pr-3 py-1.5 rounded-lg"
          style={{ background: '#111111' }}
        >
          <div
            className="w-7 h-7 rounded-full flex items-center justify-center"
            style={{ background: 'var(--accent)' }}
          >
            <UserIcon size={14} className="text-white" />
          </div>
          <span className="text-sm text-white hidden sm:inline">admin</span>
          <button
            onClick={handleLogout}
            className="ml-1 transition-colors"
            style={{ color: 'var(--text-dim)' }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = 'var(--danger)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = 'var(--text-dim)'
            }}
            title="Выйти"
          >
            <LogOut size={14} />
          </button>
        </div>
      </div>
    </header>
  )
}
