import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { changePassword, listSessions, revokeSession, revokeOtherSessions } from '../api/me'
import { ApiError } from '../api/client'
import type { Session } from '../api/types'

export function Settings() {
  return (
    <div className="flex-1 p-6 overflow-y-auto space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-white">Настройки</h2>
        <p className="text-sm text-neutral-500">Профиль, безопасность, сессии</p>
      </div>

      <PasswordSection />
      <SessionsSection />
      <PanicSection />
    </div>
  )
}

function Section({ title, description, children }: {
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <div className="bg-neutral-950 border border-neutral-800 rounded-lg">
      <div className="px-5 py-3 border-b border-neutral-800">
        <div className="text-sm font-medium text-white">{title}</div>
        {description && <div className="text-xs text-neutral-500 mt-0.5">{description}</div>}
      </div>
      <div className="p-5">{children}</div>
    </div>
  )
}

function PasswordSection() {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [saving, setSaving] = useState(false)
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setMsg('')
    setError('')
    if (newPassword !== confirm) {
      setError('Пароли не совпадают')
      return
    }
    if (newPassword.length < 12) {
      setError('Пароль должен быть не короче 12 символов')
      return
    }
    setSaving(true)
    try {
      await changePassword(oldPassword, newPassword)
      setMsg('Пароль изменён')
      setOldPassword('')
      setNewPassword('')
      setConfirm('')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Section title="Смена пароля" description="Минимум 12 символов">
      <form onSubmit={handleSubmit} className="space-y-3 max-w-md">
        <input
          type="password"
          placeholder="Текущий пароль"
          value={oldPassword}
          onChange={(e) => setOldPassword(e.target.value)}
          required
          className={inputClass}
        />
        <input
          type="password"
          placeholder="Новый пароль"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          required
          className={inputClass}
        />
        <input
          type="password"
          placeholder="Повторите новый пароль"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          required
          className={inputClass}
        />
        {error && <div className="text-sm text-red-400">{error}</div>}
        {msg && <div className="text-sm text-green-400">{msg}</div>}
        <button
          type="submit"
          disabled={saving}
          className="bg-white text-black font-medium text-sm rounded px-4 py-2 hover:bg-neutral-200 disabled:opacity-50"
        >
          {saving ? 'Сохранение...' : 'Сменить пароль'}
        </button>
      </form>
    </Section>
  )
}

function SessionsSection() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)

  async function reload() {
    setLoading(true)
    try {
      setSessions(await listSessions())
    } catch {
      /* ignore */
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [])

  async function revoke(id: string) {
    if (!confirm('Завершить эту сессию?')) return
    try {
      await revokeSession(id)
      await reload()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Ошибка')
    }
  }

  async function revokeAll() {
    if (!confirm('Завершить все сессии, кроме текущей?')) return
    try {
      await revokeOtherSessions()
      await reload()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Ошибка')
    }
  }

  return (
    <Section title="Активные сессии" description="Устройства, с которых выполнен вход">
      {loading ? (
        <div className="text-sm text-neutral-500">Загрузка...</div>
      ) : (
        <div className="space-y-2">
          {sessions.map((s) => (
            <div
              key={s.id}
              className="flex items-center justify-between border border-neutral-800 rounded px-3 py-2 text-sm"
            >
              <div>
                <div className="text-white">
                  {s.ip} {s.current && <span className="text-green-400 text-xs">· текущая</span>}
                </div>
                <div className="text-xs text-neutral-500 truncate max-w-md">{s.user_agent}</div>
                <div className="text-xs text-neutral-600">
                  Последняя активность: {new Date(s.last_seen * 1000).toLocaleString('ru-RU')}
                </div>
              </div>
              {!s.current && (
                <button
                  onClick={() => revoke(s.id)}
                  className="text-xs text-red-400 hover:text-red-300"
                >
                  Завершить
                </button>
              )}
            </div>
          ))}

          {sessions.length > 1 && (
            <button
              onClick={revokeAll}
              className="text-xs text-red-400 hover:text-red-300 mt-2"
            >
              Завершить все остальные сессии
            </button>
          )}
        </div>
      )}
    </Section>
  )
}

function PanicSection() {
  return (
    <Section
      title="Аварийное восстановление"
      description="Если вы потеряли доступ к панели"
    >
      <div className="text-sm text-neutral-400 space-y-2">
        <p>Выполните на сервере (по SSH):</p>
        <pre className="bg-neutral-900 border border-neutral-800 rounded p-3 text-xs font-mono overflow-x-auto">
{`gopanel reset-password --user admin --password "новый-пароль"

# Сбросит пароль, отключит 2FA и завершит все сессии`}
        </pre>
      </div>
    </Section>
  )
}

const inputClass =
  'w-full bg-neutral-950 border border-neutral-800 rounded px-3 py-2 text-white outline-none focus:border-neutral-600'
