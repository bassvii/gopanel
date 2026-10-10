import { LayoutDashboard, Users, Server, Activity, Settings, FileText } from 'lucide-react'
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
    <aside className="w-56 shrink-0 border-r border-neutral-800 bg-neutral-950 flex flex-col">
      <div className="px-5 py-4 border-b border-neutral-800">
        <div className="text-lg font-bold text-white">gopanel</div>
        <div className="text-xs text-neutral-500">панель управления</div>
      </div>

      <nav className="flex-1 p-2 space-y-1">
        {items.map(({ name, label, Icon }) => {
          const active = route.name === name
          return (
            <button
              key={name}
              onClick={() => navigate({ name })}
              className={
                'w-full text-left px-3 py-2 rounded flex items-center gap-3 text-sm transition-colors ' +
                (active
                  ? 'bg-neutral-800 text-white'
                  : 'text-neutral-400 hover:text-white hover:bg-neutral-900')
              }
            >
              <Icon size={16} className="shrink-0 opacity-80" />
              <span>{label}</span>
            </button>
          )
        })}
      </nav>
    </aside>
  )
}
