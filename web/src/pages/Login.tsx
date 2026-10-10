import { useState, type FormEvent } from 'react'
import { login } from '../api/auth'
import { ApiError } from '../api/client'

interface Props {
  onSuccess: () => void
}

export function Login({ onSuccess }: Props) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totpCode, setTotpCode] = useState('')
  const [needsTOTP, setNeedsTOTP] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const res = await login(username, password, totpCode || undefined)
      if (res.ok) {
        onSuccess()
        return
      }
      if (res.needs_totp) {
        setNeedsTOTP(true)
        setError('Введите код из приложения-аутентификатора')
        return
      }
      setError(res.error || 'Не удалось войти')
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Ошибка сети')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm bg-neutral-900 border border-neutral-800 rounded-lg p-6"
      >
        <h1 className="text-2xl font-bold text-white mb-1">gopanel</h1>
        <p className="text-sm text-neutral-500 mb-6">Вход в панель</p>

        <label className="block mb-4">
          <span className="text-sm text-neutral-400 block mb-1">Логин</span>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
            className="w-full bg-neutral-950 border border-neutral-800 rounded px-3 py-2 text-white outline-none focus:border-neutral-600"
          />
        </label>

        <label className="block mb-4">
          <span className="text-sm text-neutral-400 block mb-1">Пароль</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
            className="w-full bg-neutral-950 border border-neutral-800 rounded px-3 py-2 text-white outline-none focus:border-neutral-600"
          />
        </label>

        {needsTOTP && (
          <label className="block mb-4">
            <span className="text-sm text-neutral-400 block mb-1">Код 2FA</span>
            <input
              type="text"
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value)}
              inputMode="numeric"
              autoComplete="one-time-code"
              className="w-full bg-neutral-950 border border-neutral-800 rounded px-3 py-2 text-white outline-none focus:border-neutral-600"
            />
          </label>
        )}

        {error && (
          <div className="mb-4 text-sm text-red-400">{error}</div>
        )}

        <button
          type="submit"
          disabled={loading}
          className="w-full bg-white text-black font-medium rounded px-3 py-2 hover:bg-neutral-200 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {loading ? 'Вход...' : 'Войти'}
        </button>
      </form>
    </div>
  )
}
