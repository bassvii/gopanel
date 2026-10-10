import { useEffect, useState } from 'react'
import { Login } from './pages/Login'
import { Sidebar } from './components/Sidebar'
import { Header } from './components/Header'
import { me } from './api/auth'
import { RouterProvider, useRouter, type Route } from './lib/router'
import { Users } from './pages/Users'
import { Inbounds } from './pages/Inbounds'
import { Dashboard } from './pages/Dashboard'
import { Settings } from './pages/Settings'

const titles: Record<Route['name'], string> = {
  login: 'Вход',
  dashboard: 'Панель',
  users: 'Пользователи',
  inbounds: 'Инбаунды',
  traffic: 'Трафик',
  settings: 'Настройки',
  audit: 'Аудит',
}

function Page({ route }: { route: Route['name'] }) {
  switch (route) {
    case 'dashboard':
      return <Dashboard />
    case 'users':
      return <Users />
    case 'inbounds':
      return <Inbounds />
    case 'settings':
      return <Settings />
    default:
      return (
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <h2 className="text-2xl font-bold text-white mb-2">{titles[route]}</h2>
            <p className="text-neutral-500">Скоро здесь будет раздел</p>
          </div>
        </div>
      )
  }
}

function Shell({ onLogout }: { onLogout: () => void }) {
  const { route } = useRouter()

  return (
    <div className="min-h-screen flex bg-neutral-900">
      <Sidebar />
      <div className="flex-1 flex flex-col">
        <Header title={titles[route.name]} onLogout={onLogout} />
        <Page route={route.name} />
      </div>
    </div>
  )
}

function Inner() {
  const [authed, setAuthed] = useState<boolean | null>(null)

  useEffect(() => {
    me()
      .then(() => setAuthed(true))
      .catch(() => setAuthed(false))
  }, [])

  if (authed === null) {
    return (
      <div className="min-h-screen flex items-center justify-center text-neutral-500">
        Загрузка...
      </div>
    )
  }

  if (!authed) {
    return <Login onSuccess={() => setAuthed(true)} />
  }

  return <Shell onLogout={() => setAuthed(false)} />
}

export default function App() {
  return (
    <RouterProvider>
      <Inner />
    </RouterProvider>
  )
}
