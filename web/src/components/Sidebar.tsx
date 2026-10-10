import { useRouter, type Route } from '../lib/router'

const items: { name: Route['name']; label: string; icon: string }[] = [
  { name: 'dashboard', label: 'Панель', icon: '■' },
  { name: 'users', label: 'Пользователи', icon: '●' },
  { name: 'inbounds', label: 'Инбаунды', icon: '◆' },
  { name: 'traffic', label: 'Трафик', icon: '▲' },
  { name: 'settings', label: 'Настройки', icon: '⚙' },
  { name: 'audit', label: 'Аудит', icon: '≡' },
]

export function Sidebar() {
  const { route, navigate } = useRouter()

  return (
    <aside className="w-56 shrink-0 border-r border-neutral-800 bg-neutral-950 flex flex-col">
      <div className="px-5 py-4 border-b border-neutral-800">
        <div className="text-lg font-bold text-white">gopanel</div>
        <div className="text-xs text-neutral-500">панель управления</div>
      </div>

      <nav className="flex-1 p-2">
        {items.map((item) => {
          const active = route.name === item.name
          return (
            <button
              key={item.name}
              onClick={() => navigate({ name: item.name })}
              className={
                'w-full text-left px-3 py-2 rounded flex items-center gap-3 text-sm transition-colors ' +
                (active
                  ? 'bg-neutral-800 text-white'
                  : 'text-neutral-400 hover:text-white hover:bg-neutral-900')
              }
            >
              <span className="w-4 text-center opacity-60">{item.icon}</span>
              <span>{item.label}</span>
            </button>
          )
        })}
      </nav>
    </aside>
  )
}
