import { User as UserIcon, Settings, LogOut, KeyRound } from 'lucide-react'
import { Dropdown } from './ui'
import { useRouter } from '../lib/router'
import { logout } from '../api/auth'

interface Props {
  username: string
  onLogout: () => void
}

export function ProfileMenu({ username, onLogout }: Props) {
  const router = useRouter()

  async function handleLogout() {
    try {
      await logout()
    } finally {
      onLogout()
    }
  }

  return (
    <Dropdown
      align="right"
      width={220}
      trigger={() => (
        <div
          className="flex items-center gap-2.5 pl-1.5 pr-3 py-1.5 rounded-lg transition-colors cursor-pointer"
          style={{ background: '#111111' }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = '#1a1a1a'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = '#111111'
          }}
        >
          <div
            className="w-7 h-7 rounded-full flex items-center justify-center shrink-0"
            style={{ background: 'var(--accent)' }}
          >
            <UserIcon size={14} className="text-white" />
          </div>
          <span className="text-sm text-white">{username}</span>
        </div>
      )}
    >
      {/* Информация */}
      <div
        className="px-4 py-3"
        style={{ borderBottom: '1px solid var(--border)' }}
      >
        <div className="text-sm text-white">{username}</div>
        <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
          Администратор
        </div>
      </div>

      {/* Пункты меню */}
      <MenuItem
        icon={<Settings size={14} />}
        label="Настройки"
        onClick={() => router.navigate({ name: 'settings' })}
      />
      <MenuItem
        icon={<KeyRound size={14} />}
        label="Сессии и пароль"
        onClick={() => router.navigate({ name: 'settings' })}
      />
      <MenuItem
        icon={<LogOut size={14} />}
        label="Выйти"
        onClick={handleLogout}
        danger
      />
    </Dropdown>
  )
}

function MenuItem({
  icon,
  label,
  onClick,
  danger,
}: {
  icon: React.ReactNode
  label: string
  onClick: () => void
  danger?: boolean
}) {
  const color = danger ? 'var(--danger)' : 'var(--text-muted)'
  return (
    <button
      onClick={onClick}
      className="w-full text-left px-4 py-2.5 flex items-center gap-3 text-sm transition-colors"
      style={{ color, borderBottom: '1px solid var(--border)' }}
      onMouseEnter={(e) => {
        e.currentTarget.style.background = 'var(--bg-surface-hover)'
        e.currentTarget.style.color = danger ? 'var(--danger)' : 'var(--text)'
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.background = 'transparent'
        e.currentTarget.style.color = color
      }}
    >
      {icon}
      {label}
    </button>
  )
}
