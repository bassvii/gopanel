import {
  LayoutDashboard,
  Users,
  Server,
  Activity,
  Settings,
  FileText,
  Shield,
} from 'lucide-react'
import { useRouter, type Route } from '../lib/router'

const items: { name: Route['name']; label: string; Icon: typeof Users }[] = [
  { name: 'dashboard', label: 'Панель', Icon: LayoutDashboard },
  { name: 'users', label: 'Пользователи', Icon: Users },
  { name: 'inbounds', label: 'Инбаунды', Icon: Server },
  { name: 'traffic', label: 'Трафик', Icon: Activity },
  { name: 'settings', label: 'Настройки', Icon: Settings },
  { name: 'audit', label: 'Аудит', Icon: FileText },
]

export function Sidebar() {
  const { route, navigate } = useRouter()

  return (
    <aside
      className="w-60 shrink-0 flex flex-col"
      style={{ background: '#0a0a0a', borderRight: '1px solid var(--border)' }}
    >
      {/* Логотип */}
      <div className="h-16 px-5 flex items-center" style={{ borderBottom: '1px solid var(--border)' }}>
        <div className="flex items-center gap-3">
          <div
            className="w-9 h-9 rounded-lg flex items-center justify-center"
            style={{ background: 'var(--accent)' }}
          >
            <Shield size={18} className="text-white" />
          </div>
          <div>
            <div className="text-base font-bold text-white leading-tight">gopanel</div>
            <div className="text-xs" style={{ color: 'var(--text-dim)' }}>
              VPN Management
            </div>
          </div>
        </div>
      </div>

      {/* Навигация */}
      <nav className="flex-1 p-3 space-y-1">
        {items.map(({ name, label, Icon }) => {
          const active = route.name === name
          return (
            <button
              key={name}
              onClick={() => navigate({ name })}
              className="w-full text-left px-3 py-2.5 rounded-lg flex items-center gap-3 text-sm font-medium transition-colors"
              style={{
                background: active ? 'var(--accent-bg)' : 'transparent',
                color: active ? 'var(--accent)' : 'var(--text-muted)',
              }}
              onMouseEnter={(e) => {
                if (!active) {
                  e.currentTarget.style.background = '#111111'
                  e.currentTarget.style.color = 'var(--text)'
                }
              }}
              onMouseLeave={(e) => {
                if (!active) {
                  e.currentTarget.style.background = 'transparent'
                  e.currentTarget.style.color = 'var(--text-muted)'
                }
              }}
            >
              <Icon size={18} className="shrink-0" />
              <span>{label}</span>
            </button>
          )
        })}
      </nav>

      {/* Низ — версия и брендинг */}
      <div className="p-3 space-y-2" style={{ borderTop: '1px solid var(--border)' }}>
        <div className="px-3 py-2 flex items-center gap-2">
          <div
            className="w-1.5 h-1.5 rounded-full"
            style={{ background: 'var(--success)' }}
          />
          <span className="text-xs" style={{ color: 'var(--text-dim)' }}>
            Версия панели
          </span>
          <span className="text-xs ml-auto" style={{ color: 'var(--text-muted)' }}>
            v0.1
          </span>
        </div>
        <div className="px-3 py-2 flex items-center gap-2">
          <Shield size={14} style={{ color: 'var(--accent)' }} />
          <div>
            <div className="text-xs font-medium text-white">gopanel</div>
            <div className="text-xs" style={{ color: 'var(--text-dim)' }}>
              The best VPN panel
            </div>
          </div>
        </div>
      </div>
    </aside>
  )
}
