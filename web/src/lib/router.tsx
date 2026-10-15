// SPDX-License-Identifier: AGPL-3.0-or-later

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react'
import { getBasePath } from './basePath'

export type Route =
  | { name: 'login' }
  | { name: 'dashboard' }
  | { name: 'users' }
  | { name: 'inbounds' }
  | { name: 'routing' }
  | { name: 'traffic' }
  | { name: 'settings' }
  | { name: 'audit' }

interface RouterCtx {
  route: Route
  navigate: (r: Route) => void
}

const Ctx = createContext<RouterCtx | null>(null)

// knownRoutes — URL-сегменты, которые роутер понимает.
// Всё, что не здесь — 404.
const knownRoutes: Route['name'][] = [
  'dashboard',
  'users',
  'inbounds',
  'routing',
  'traffic',
  'settings',
  'audit',
]

// parsePath извлекает route из текущего URL.
// URL: /<base>/<segment>?<query>#<hash>
function parsePath(): Route {
  const base = getBasePath()
  let path = window.location.pathname
  if (base && path.startsWith(base)) {
    path = path.slice(base.length)
  }
  const parts = path.split('/').filter(Boolean)
  const first = parts[0] || 'dashboard'

  if (knownRoutes.includes(first as Route['name'])) {
    return { name: first as Route['name'] }
  }
  // Неизвестный путь — возвращаем "dashboard", а NotFound
  // обрабатывается в App.tsx через флаг.
  return { name: 'dashboard' }
}

// isKnownPath сообщает, распознан ли URL роутером.
// Используется в App.tsx, чтобы показать NotFound.
export function isKnownPath(): boolean {
  const base = getBasePath()
  let path = window.location.pathname
  if (base && path.startsWith(base)) {
    path = path.slice(base.length)
  }
  const parts = path.split('/').filter(Boolean)
  if (parts.length === 0) return true // корень = dashboard
  const first = parts[0]
  return knownRoutes.includes(first as Route['name'])
}

export function RouterProvider({ children }: { children: ReactNode }) {
  const [route, setRoute] = useState<Route>(parsePath)

  // Синхронизация с URL при popstate (кнопки назад/вперёд).
  useEffect(() => {
    function onPop() {
      setRoute(parsePath())
    }
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  const navigate = useCallback((r: Route) => {
    const base = getBasePath()
    const path = r.name === 'dashboard' ? '' : '/' + r.name
    window.history.pushState({}, '', base + path + '/')
    setRoute(r)
  }, [])

  return (
    <Ctx.Provider value={{ route, navigate }}>
      {children}
    </Ctx.Provider>
  )
}

export function useRouter(): RouterCtx {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useRouter must be used inside RouterProvider')
  return ctx
}
