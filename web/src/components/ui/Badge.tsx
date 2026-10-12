import type { ReactNode } from 'react'

type Variant = 'success' | 'danger' | 'warning' | 'neutral'

interface Props {
  variant?: Variant
  children: ReactNode
}

const styles: Record<Variant, { color: string; background: string }> = {
  success: { color: 'var(--success)', background: 'rgba(34, 197, 94, 0.1)' },
  danger: { color: 'var(--danger)', background: 'rgba(239, 68, 68, 0.1)' },
  warning: { color: 'var(--warning)', background: 'rgba(234, 179, 8, 0.1)' },
  neutral: { color: 'var(--text-muted)', background: '#1a1a1a' },
}

export function Badge({ variant = 'neutral', children }: Props) {
  const s = styles[variant]
  return (
    <span
      className="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded"
      style={{ color: s.color, background: s.background }}
    >
      {children}
    </span>
  )
}
