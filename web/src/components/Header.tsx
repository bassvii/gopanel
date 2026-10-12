import { NotificationsBell } from './NotificationsBell'
import { ProfileMenu } from './ProfileMenu'

interface Props {
  title: string
  onLogout: () => void
}

export function Header({ title, onLogout }: Props) {
  return (
    <header
      className="h-16 flex items-center justify-between px-6 gap-4"
      style={{
        background: 'var(--bg-surface)',
        borderBottom: '1px solid var(--border)',
      }}
    >
      <h1 className="text-lg font-medium text-white">{title}</h1>

      <div className="flex items-center gap-2">
        <div
          className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs"
          style={{ background: '#111111' }}
        >
          <div className="w-2 h-2 rounded-full" style={{ background: 'var(--success)' }} />
          <span style={{ color: 'var(--text-muted)' }}>Сервер онлайн</span>
        </div>

        <NotificationsBell />
        <ProfileMenu username="admin" onLogout={onLogout} />
      </div>
    </header>
  )
}
