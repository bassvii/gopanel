import { useEffect, useState } from 'react'
import { Login } from './pages/Login'
import { me } from './api/auth'
import { RouterProvider, useRouter } from './lib/router'

function Placeholder({ title }: { title: string }) {
  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <h1 className="text-3xl font-bold text-white mb-2">{title}</h1>
        <p className="text-neutral-500">Скоро здесь будет раздел</p>
      </div>
    </div>
  )
}

function Shell() {
  const { route } = useRouter()

  switch (route.name) {
    case 'dashboard':
      return <Placeholder title="Панель" />
    case 'users':
      return <Placeholder title="Пользователи" />
    case 'inbounds':
      return <Placeholder title="Инбаунды" />
    case 'traffic':
      return <Placeholder title="Трафик" />
    case 'settings':
      return <Placeholder title="Настройки" />
    case 'audit':
      return <Placeholder title="Аудит" />
    default:
      return <Placeholder title="Не найдено" />
  }
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

  return <Shell />
}

export default function App() {
  return (
    <RouterProvider>
      <Inner />
    </RouterProvider>
  )
}
