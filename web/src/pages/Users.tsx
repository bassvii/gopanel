import { useEffect, useState, type FormEvent } from 'react'
import { Modal } from '../components/Modal'
import { formatBytes, formatDate, dateInputFromUnix, unixFromDateInput } from '../lib/format'
import {
  listUsers,
  createUser,
  updateUser,
  deleteUser,
  getUserLinks,
  type CreateUserInput,
} from '../api/users'
import { ApiError } from '../api/client'
import type { LinkEntry, User } from '../api/types'
import { listInbounds } from '../api/inbounds'
import { attachUserToInbound } from '../api/users'
import type { Inbound } from '../api/types'


export function Users() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [editing, setEditing] = useState<User | null>(null)
  const [creating, setCreating] = useState(false)
  const [linksFor, setLinksFor] = useState<User | null>(null)

  async function reload() {
    setLoading(true)
    setError('')
    try {
      setUsers(await listUsers())
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка загрузки')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [])

  async function handleDelete(u: User) {
    if (!confirm(`Удалить пользователя «${u.name}»?`)) return
    try {
      await deleteUser(u.id)
      await reload()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Ошибка удаления')
    }
  }

  return (
    <div className="flex-1 p-6 overflow-y-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold text-white">Пользователи</h2>
          <p className="text-sm text-neutral-500">
            {users.length} {users.length === 1 ? 'запись' : 'записей'}
          </p>
        </div>
        <button
          onClick={() => setCreating(true)}
          className="bg-white text-black font-medium text-sm rounded px-4 py-2 hover:bg-neutral-200"
        >
          + Создать
        </button>
      </div>

      {error && (
        <div className="mb-4 text-sm text-red-400 bg-red-950/30 border border-red-900 rounded px-3 py-2">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-neutral-500">Загрузка...</div>
      ) : users.length === 0 ? (
        <div className="text-neutral-500 text-center py-16">
          Пользователей пока нет. Создайте первого.
        </div>
      ) : (
        <div className="border border-neutral-800 rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-neutral-950 text-neutral-400">
              <tr>
                <th className="text-left px-4 py-3 font-medium">Имя</th>
                <th className="text-left px-4 py-3 font-medium">Email</th>
                <th className="text-left px-4 py-3 font-medium">Трафик</th>
                <th className="text-left px-4 py-3 font-medium">Срок</th>
                <th className="text-left px-4 py-3 font-medium">Статус</th>
                <th className="text-right px-4 py-3 font-medium">Действия</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-t border-neutral-800 hover:bg-neutral-900/40">
                  <td className="px-4 py-3 text-white">{u.name}</td>
                  <td className="px-4 py-3 text-neutral-400">{u.email}</td>
                  <td className="px-4 py-3 text-neutral-400">
                    {formatBytes(u.used_bytes)}
                    {u.quota_bytes > 0 && ` / ${formatBytes(u.quota_bytes)}`}
                  </td>
                  <td className="px-4 py-3 text-neutral-400">{formatDate(u.expires_at)}</td>
                  <td className="px-4 py-3">
                    {u.enabled ? (
                      <span className="text-green-400 text-xs">включён</span>
                    ) : (
                      <span className="text-red-400 text-xs">отключён</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => setLinksFor(u)}
                      className="text-neutral-400 hover:text-white text-xs mr-3"
                    >
                      Ссылки
                    </button>
                    <button
                      onClick={() => setEditing(u)}
                      className="text-neutral-400 hover:text-white text-xs mr-3"
                    >
                      Изменить
                    </button>
                    <button
                      onClick={() => handleDelete(u)}
                      className="text-red-400 hover:text-red-300 text-xs"
                    >
                      Удалить
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {creating && (
        <UserForm
          onClose={() => setCreating(false)}
          onSaved={async () => {
            setCreating(false)
            await reload()
          }}
        />
      )}

      {editing && (
        <UserForm
          user={editing}
          onClose={() => setEditing(null)}
          onSaved={async () => {
            setEditing(null)
            await reload()
          }}
        />
      )}

      {linksFor && (
        <LinksModal user={linksFor} onClose={() => setLinksFor(null)} />
      )}
    </div>
  )
}

// --- Форма пользователя ---

interface UserFormProps {
  user?: User
  onClose: () => void
  onSaved: () => void
}

function UserForm({ user, onClose, onSaved }: UserFormProps) {
  const isEdit = !!user
  const [name, setName] = useState(user?.name ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [enabled, setEnabled] = useState(user?.enabled ?? true)
  const [quotaGB, setQuotaGB] = useState(
    user?.quota_bytes ? String(Math.round(user.quota_bytes / 1024 / 1024 / 1024)) : '0',
  )
  const [expires, setExpires] = useState(dateInputFromUnix(user?.expires_at ?? 0))
  const [deviceLimit, setDeviceLimit] = useState(String(user?.device_limit ?? 0))
  const [note, setNote] = useState(user?.note ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSaving(true)

    const input: CreateUserInput = {
      name,
      email: email || undefined,
      enabled,
      quota_bytes: Math.round(Number(quotaGB) * 1024 * 1024 * 1024),
      expires_at: unixFromDateInput(expires),
      device_limit: Number(deviceLimit) || 0,
      note,
    }

    try {
      if (isEdit && user) {
        await updateUser(user.id, input)
      } else {
        await createUser(input)
      }
      onSaved()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка сохранения')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      title={isEdit ? `Изменить «${user?.name}»` : 'Новый пользователь'}
      onClose={onClose}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <Field label="Имя">
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className="input"
          />
        </Field>

        <Field label="Email (для статистики Xray)">
          <input
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="оставьте пустым — сгенерируется"
            className="input"
          />
        </Field>

        <Field label="Лимит трафика (ГБ, 0 — без лимита)">
          <input
            type="number"
            value={quotaGB}
            onChange={(e) => setQuotaGB(e.target.value)}
            min={0}
            className="input"
          />
        </Field>

        <Field label="Срок действия">
          <input
            type="date"
            value={expires}
            onChange={(e) => setExpires(e.target.value)}
            className="input"
          />
        </Field>

        <Field label="Лимит устройств (0 — без лимита)">
          <input
            type="number"
            value={deviceLimit}
            onChange={(e) => setDeviceLimit(e.target.value)}
            min={0}
            className="input"
          />
        </Field>

        <Field label="Заметка">
          <input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className="input"
          />
        </Field>

        <label className="flex items-center gap-2 text-sm text-neutral-300">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          Включён
        </label>

        {error && <div className="text-sm text-red-400">{error}</div>}

        <div className="flex justify-end gap-2 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="text-sm text-neutral-400 hover:text-white px-4 py-2"
          >
            Отмена
          </button>
          <button
            type="submit"
            disabled={saving}
            className="bg-white text-black font-medium text-sm rounded px-4 py-2 hover:bg-neutral-200 disabled:opacity-50"
          >
            {saving ? 'Сохранение...' : 'Сохранить'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-sm text-neutral-400 block mb-1">{label}</span>
      {children}
    </label>
  )
}

// --- Модальное окно со ссылками ---

function LinksModal({ user, onClose }: { user: User; onClose: () => void }) {
  const [address, setAddress] = useState(localStorage.getItem('gopanel_address') ?? '')
  const [links, setLinks] = useState<LinkEntry[]>([])
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listInbounds().then(setInbounds).catch(() => {})
  }, [])

  async function load() {
    if (!address) {
      setError('Укажите адрес сервера')
      return
    }
    localStorage.setItem('gopanel_address', address)
    setLoading(true)
    setError('')
    try {
      setLinks(await getUserLinks(user.id, address, false))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Ошибка загрузки')
    } finally {
      setLoading(false)
    }
  }

  async function attach(inboundId: number) {
    try {
      await attachUserToInbound(user.id, inboundId)
      await load()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'Ошибка привязки')
    }
  }

  useEffect(() => {
    if (address) load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const attachedIds = new Set(links.map((l) => l.inbound_id))
  const available = inbounds.filter((inb) => !attachedIds.has(inb.id))

  return (
    <Modal title={`Ссылки для «${user.name}»`} onClose={onClose} wide>
      <div className="space-y-4">
        <Field label="Адрес сервера (IP или домен)">
          <div className="flex gap-2">
            <input
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              placeholder="1.2.3.4 или example.com"
              className="w-full bg-neutral-950 border border-neutral-800 rounded px-3 py-2 text-white outline-none focus:border-neutral-600 flex-1"
            />
            <button
              onClick={() => load()}
              className="bg-white text-black font-medium text-sm rounded px-4 py-2 hover:bg-neutral-200"
            >
              Загрузить
            </button>
          </div>
        </Field>

        {error && <div className="text-sm text-red-400">{error}</div>}
        {loading && <div className="text-neutral-500 text-sm">Загрузка...</div>}

        {links.length === 0 && !loading && !error && (
          <div className="text-neutral-500 text-sm text-center py-4">
            У пользователя нет привязанных инбаундов.
          </div>
        )}

        {links.map((l) => (
          <div key={l.inbound_id} className="border border-neutral-800 rounded p-3 space-y-2">
            <div className="flex items-center justify-between">
              <div className="text-sm text-white">
                {l.protocol} <span className="text-neutral-500">· {l.tag}</span>
              </div>
              <button
                onClick={() => navigator.clipboard.writeText(l.url)}
                className="text-xs text-neutral-400 hover:text-white"
              >
                Копировать
              </button>
            </div>
            {l.error ? (
              <div className="text-xs text-red-400">{l.error}</div>
            ) : (
              <div className="text-xs text-neutral-500 break-all font-mono">{l.url}</div>
            )}
          </div>
        ))}

        {available.length > 0 && (
          <div className="border-t border-neutral-800 pt-4">
            <div className="text-sm text-neutral-400 mb-2">Привязать инбаунд:</div>
            <div className="flex flex-wrap gap-2">
              {available.map((inb) => (
                <button
                  key={inb.id}
                  onClick={() => attach(inb.id)}
                  className="text-xs bg-neutral-800 hover:bg-neutral-700 text-white rounded px-3 py-1.5"
                >
                  + {inb.tag} ({inb.protocol})
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </Modal>
  )
}
