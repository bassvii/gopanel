// SPDX-License-Identifier: AGPL-3.0-or-later

import { Home } from 'lucide-react'
import { useRouter } from '../lib/router'

export function NotFound() {
  const { navigate } = useRouter()

  return (
    <div className="flex-1 flex items-center justify-center p-6">
      <div className="text-center max-w-md">
        <h1
          className="text-7xl font-bold mb-4"
          style={{ color: 'var(--text-dim)' }}
        >
          404
        </h1>
        <h2 className="text-xl font-medium text-white mb-2">
          Страница не найдена
        </h2>
        <p className="text-sm mb-6" style={{ color: 'var(--text-muted)' }}>
          Возможно, ссылка устарела или вы ошиблись в адресе.
        </p>
        <button
          onClick={() => navigate({ name: 'dashboard' })}
          className="inline-flex items-center gap-2 text-sm font-medium px-4 py-2 rounded-lg transition-colors"
          style={{ background: 'var(--accent)', color: 'white' }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = 'var(--accent-hover)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = 'var(--accent)'
          }}
        >
          <Home size={16} />
          На главную
        </button>
      </div>
    </div>
  )
}
