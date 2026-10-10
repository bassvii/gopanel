import { createContext, useContext, useState, type ReactNode } from 'react'

export type Route =
  | { name: 'login' }
  | { name: 'dashboard' }
  | { name: 'users' }
  | { name: 'inbounds' }
  | { name: 'traffic' }
  | { name: 'settings' }
  | { name: 'audit' }

interface RouterCtx {
  route: Route
  navigate: (r: Route) => void
}

const Ctx = createContext<RouterCtx | null>(null)

export function RouterProvider({ children }: { children: ReactNode }) {
  const [route, setRoute] = useState<Route>({ name: 'dashboard' })
  return (
    <Ctx.Provider value={{ route, navigate: setRoute }}>
      {children}
    </Ctx.Provider>
  )
}

export function useRouter(): RouterCtx {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useRouter must be used inside RouterProvider')
  return ctx
}
