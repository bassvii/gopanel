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
    <header className="h-14 border-b border-neutral-800 bg-neutral-950 flex items-center justify-between px-6">
      <h1 className="text-lg font-medium text-white">{title}</h1>

      <button
        onClick={handleLogout}
        className="text-sm text-neutral-400 hover:text-white px-3 py-1.5 rounded hover:bg-neutral-900"
      >
        Выйти
      </button>
    </header>
  )
}
