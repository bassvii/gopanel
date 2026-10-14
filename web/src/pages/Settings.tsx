import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Copy, KeyRound, LogOut, Shield, ShieldCheck, ShieldOff } from 'lucide-react'
import {
  changePassword,
  listSessions,
  revokeSession,
  revokeOtherSessions,
  getTOTPStatus,
  setupTOTP,
  enableTOTP,
  disableTOTP,
  type TOTPSetupResponse,
} from '../api/me'
import { downloadBackup, saveBlob, defaultBackupName } from '../api/backup'
import { ApiError } from '../api/client'
import { toast } from 'sonner'
import { Button, Input } from '../components/ui'
import type { Session } from '../api/types'
import { getBasePath } from '../lib/basePath'
import { useConfirm } from '../hooks/useConfirm'

export function Settings() {
  return (
    <div className="flex-1 p-6 overflow-y-auto space-y-6">
      {/* Заголовок */}
      <div>
        <h2 className="text-2xl font-bold text-white">Настройки</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-muted)' }}>
          Профиль, безопасность, сессии
        </p>
      </div>

      <PasswordSection />
      <TOTPSection />
      <SessionsSection />
      <BackupSection />
      <PanicSection />
    </div>
  )
}

// --- Секция (карточка) ---

function Section({
  title,
  description,
  icon,
  children,
}: {
  title: string
  description?: string
  icon?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="card">
      <div
        className="px-5 py-4 flex items-center gap-3"
        style={{ borderBottom: '1px solid var(--border)' }}
      >
        {icon && (
          <div
            className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
            style={{ background: 'var(--accent-bg)', color: 'var(--accent)' }}
          >
            {icon}
          </div>
        )}
        <div>
          <div className="text-sm font-medium text-white">{title}</div>
          {description && (
            <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
              {description}
            </div>
          )}
        </div>
      </div>
      <div className="p-5">{children}</div>
    </div>
  )
}

// --- Смена пароля ---

function PasswordSection() {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
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
      toast.success('Пароль изменён')
      setOldPassword('')
      setNewPassword('')
      setConfirm('')
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Ошибка'
      setError(msg)
      toast.error(msg)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Section
      title="Смена пароля"
      description="Минимум 12 символов"
      icon={<KeyRound size={16} />}
    >
      <form onSubmit={handleSubmit} className="space-y-3 max-w-md">
        <Input
          type="password"
          placeholder="Текущий пароль"
          value={oldPassword}
          onChange={(e) => setOldPassword(e.target.value)}
          required
          autoComplete="current-password"
        />
        <Input
          type="password"
          placeholder="Новый пароль"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          required
          autoComplete="new-password"
        />
        <Input
          type="password"
          placeholder="Повторите новый пароль"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          required
          autoComplete="new-password"
          error={error}
        />
        <Button type="submit" disabled={saving}>
          {saving ? 'Сохранение...' : 'Сменить пароль'}
        </Button>
      </form>
    </Section>
  )
}

// --- Сессии ---

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
      toast.success('Сессия завершена')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка')
    }
  }

  async function revokeAll() {
    if (!confirm('Завершить все сессии, кроме текущей?')) return
    try {
      await revokeOtherSessions()
      await reload()
      toast.success('Все остальные сессии завершены')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка')
    }
  }

  return (
    <Section
      title="Активные сессии"
      description="Устройства, с которых выполнен вход"
      icon={<LogOut size={16} />}
    >
      {loading ? (
        <div className="text-sm" style={{ color: 'var(--text-muted)' }}>
          Загрузка...
        </div>
      ) : (
        <div className="space-y-2">
          {sessions.map((s) => (
            <div
              key={s.id}
              className="flex items-center justify-between rounded-lg px-4 py-3"
              style={{ background: '#0a0a0a', border: '1px solid var(--border)' }}
            >
              <div className="min-w-0 flex-1">
                <div className="text-sm text-white">
                  {s.ip}{' '}
                  {s.current && (
                    <span className="text-xs" style={{ color: 'var(--success)' }}>
                      · текущая
                    </span>
                  )}
                </div>
                <div
                  className="text-xs truncate max-w-md mt-0.5"
                  style={{ color: 'var(--text-muted)' }}
                >
                  {s.user_agent}
                </div>
                <div className="text-xs mt-0.5" style={{ color: 'var(--text-dim)' }}>
                  Последняя активность:{' '}
                  {new Date(s.last_seen * 1000).toLocaleString('ru-RU')}
                </div>
              </div>
              {!s.current && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => revoke(s.id)}
                >
                  Завершить
                </Button>
              )}
            </div>
          ))}

          {sessions.length > 1 && (
            <Button variant="ghost" size="sm" onClick={revokeAll}>
              Завершить все остальные
            </Button>
          )}
        </div>
      )}
    </Section>
  )
}

// --- Бэкап ---

// --- Бэкап ---

function BackupSection() {
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function handleBackup(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!password) {
      setError('Введите пароль')
      return
    }
    setBusy(true)
    try {
      const blob = await downloadBackup(password)
      saveBlob(blob, defaultBackupName())
      toast.success('Бэкап скачан')
      setPassword('')
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Ошибка'
      setError(msg === 'invalid password' ? 'Неверный пароль' : msg)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Section
      title="Резервные копии"
      description="Скачать дамп базы данных"
      icon={<Shield size={16} />}
    >
      <div className="space-y-3 max-w-md">
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
          Дамп содержит пользователей, инбаунды, настройки, TOTP-секреты
          (зашифрованные) и журнал аудита. Для скачивания требуется пароль.
        </p>
        <form onSubmit={handleBackup} className="space-y-3">
          <Input
            type="password"
            placeholder="Пароль администратора"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            error={error}
          />
          <Button type="submit" disabled={busy}>
            {busy ? 'Готовлю бэкап...' : 'Скачать бэкап'}
          </Button>
        </form>
      </div>
    </Section>
  )
}

// --- Аварийное восстановление ---

function PanicSection() {
  return (
    <Section
      title="Аварийное восстановление"
      description="Если вы потеряли доступ к панели"
      icon={<KeyRound size={16} />}
    >
      <div className="space-y-3">
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
          Выполните на сервере по SSH:
        </p>
        <pre
          className="rounded-lg p-4 text-xs font-mono overflow-x-auto"
          style={{
            background: '#0a0a0a',
            border: '1px solid var(--border)',
            color: 'var(--text-muted)',
          }}
        >
          {`gopanel reset-password --user admin

          # Спросит новый пароль интерактивно (без эха).
          # Сбросит пароль, отключит 2FA и завершит все сессии.`}
        </pre>
      </div>
    </Section>
  )
}


// --- Двухфакторная аутентификация ---

function TOTPSection() {
  const [enabled, setEnabled] = useState<boolean | null>(null)
  const [loading, setLoading] = useState(true)
  const [setupData, setSetupData] = useState<TOTPSetupResponse | null>(null)
  const [code, setCode] = useState('')
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [disablePassword, setDisablePassword] = useState('')
  const [disableCode, setDisableCode] = useState('')
  const [disableError, setDisableError] = useState('')
  const { confirm, dialog } = useConfirm()

  async function reload() {
    setLoading(true)
    try {
      setEnabled(await getTOTPStatus())
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка загрузки статуса 2FA')
      setEnabled(false)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [])

  async function handleSetup() {
    setError('')
    setBusy(true)
    try {
      setSetupData(await setupTOTP())
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка настройки 2FA')
    } finally {
      setBusy(false)
    }
  }

  async function handleEnable() {
    if (!code) {
      setError('Введите код из приложения')
      return
    }
    setError('')
    setBusy(true)
    try {
      const res = await enableTOTP(code)
      if (!res.ok) {
        setError(res.error || 'Неверный код')
        return
      }
      setRecoveryCodes(res.recovery_codes ?? [])
      setSetupData(null)
      setCode('')
      setEnabled(true)
      toast.success('2FA включена')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка')
    } finally {
      setBusy(false)
    }
  }

  async function handleDisable() {
    if (!disablePassword) {
      setDisableError('Введите пароль')
      return
    }
    if (!disableCode) {
      setDisableError('Введите код из приложения')
      return
    }

    const ok = await confirm({
      title: 'Отключить 2FA?',
      message:
        'После отключения вход в панель будет защищён только паролем. Это снижает безопасность.',
      confirmLabel: 'Отключить',
      danger: true,
    })
    if (!ok) return

    setBusy(true)
    setDisableError('')
    try {
      await disableTOTP(disablePassword, disableCode)
      setEnabled(false)
      setSetupData(null)
      setCode('')
      setRecoveryCodes(null)
      setDisablePassword('')
      setDisableCode('')
      toast.success('2FA отключена')
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Ошибка'
      setDisableError(msg === 'invalid credentials' ? 'Неверный пароль или код' : msg)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Section
      title="Двухфакторная аутентификация"
      description="Дополнительная защита входа через TOTP-приложение"
      icon={<ShieldCheck size={16} />}
    >
      {loading ? (
        <div className="text-sm" style={{ color: 'var(--text-muted)' }}>
          Загрузка...
        </div>
      ) : enabled ? (
        <div className="space-y-3 max-w-md">
          <div>
            <span
              className="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded"
              style={{ color: 'var(--success)', background: 'rgba(34, 197, 94, 0.1)' }}
            >
              <ShieldCheck size={12} />
              Включена
            </span>
          </div>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Для отключения введите пароль и текущий код из приложения.
          </p>
          <Input
            type="password"
            placeholder="Пароль администратора"
            value={disablePassword}
            onChange={(e) => setDisablePassword(e.target.value)}
            autoComplete="current-password"
          />
          <Input
            placeholder="Код из приложения"
            value={disableCode}
            onChange={(e) => setDisableCode(e.target.value)}
            inputMode="numeric"
            autoComplete="one-time-code"
            error={disableError}
          />
          <Button variant="ghost" size="sm" onClick={handleDisable} disabled={busy}>
            <ShieldOff size={14} />
            Отключить 2FA
          </Button>
        </div>
      ) : setupData ? (
        <div className="space-y-4 max-w-md">
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Отсканируйте QR в приложении-аутентификаторе (Google Authenticator, Aegis,
            1Password) или введите секрет вручную.
          </p>

          <div className="flex justify-center">
            <img
              src={`${getBasePath()}/2fa/qr?url=${encodeURIComponent(setupData.otpauth_url)}`}
              alt="QR-код"
              className="bg-white p-2 rounded-lg"
              width={200}
              height={200}
            />
          </div>

          <label className="block">
            <span className="text-sm text-neutral-400 block mb-1">
              Секрет (если QR не считывается)
            </span>
            <div className="flex gap-2">
              <input
                value={setupData.secret}
                readOnly
                className="input font-mono text-xs flex-1"
              />
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => {
                  navigator.clipboard.writeText(setupData.secret)
                  toast.success('Секрет скопирован')
                }}
              >
                <Copy size={14} />
              </Button>
            </div>
          </label>

          <label className="block">
            <span className="text-sm text-neutral-400 block mb-1">Код из приложения</span>
            <input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              inputMode="numeric"
              autoComplete="one-time-code"
              placeholder="6 цифр"
              className="input font-mono"
            />
          </label>

          {error && <div className="text-sm text-red-400">{error}</div>}

          <div className="flex gap-2">
            <Button
              variant="ghost"
              onClick={() => {
                setSetupData(null)
                setCode('')
                setError('')
              }}
            >
              Отмена
            </Button>
            <Button onClick={handleEnable} disabled={busy}>
              {busy ? 'Проверка...' : 'Включить 2FA'}
            </Button>
          </div>
        </div>
      ) : recoveryCodes ? (
        <div className="space-y-3 max-w-md">
          <p className="text-sm" style={{ color: 'var(--warning)' }}>
            Сохраните коды восстановления. Они показываются <b>один раз</b>. Каждый код
            одноразовый.
          </p>
          <div className="grid grid-cols-2 gap-2">
            {recoveryCodes.map((c) => (
              <code
                key={c}
                className="text-xs font-mono p-2 rounded"
                style={{ background: '#0a0a0a', border: '1px solid var(--border)' }}
              >
                {c}
              </code>
            ))}
          </div>
          <div className="flex gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                navigator.clipboard.writeText(recoveryCodes.join('\n'))
                toast.success('Коды скопированы')
              }}
            >
              <Copy size={14} />
              Копировать все
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setRecoveryCodes(null)}>
              Готово
            </Button>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            2FA не включена. Рекомендуется включить для защиты панели.
          </p>
          {error && <div className="text-sm text-red-400">{error}</div>}
          <Button onClick={handleSetup} disabled={busy}>
            {busy ? 'Подготовка...' : 'Включить 2FA'}
          </Button>
        </div>
      )}

      {dialog}
    </Section>
  )
}
