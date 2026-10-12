import type { ReactNode } from 'react'

interface CardProps {
  children: ReactNode
  className?: string
  padding?: 'none' | 'sm' | 'md' | 'lg'
}

const paddingClasses: Record<NonNullable<CardProps['padding']>, string> = {
  none: '',
  sm: 'p-3',
  md: 'p-5',
  lg: 'p-6',
}

export function Card({ children, className = '', padding = 'md' }: CardProps) {
  return (
    <div
      className={`rounded-xl ${paddingClasses[padding]} ${className}`}
      style={{
        background: 'var(--bg-surface)',
        border: '1px solid var(--border)',
      }}
    >
      {children}
    </div>
  )
}

export function CardHeader({ children }: { children: ReactNode }) {
  return (
    <div
      className="px-5 py-3 flex items-center justify-between"
      style={{ borderBottom: '1px solid var(--border)' }}
    >
      {children}
    </div>
  )
}

export function CardTitle({ children }: { children: ReactNode }) {
  return <div className="text-sm font-medium text-white">{children}</div>
}

export function CardBody({ children }: { children: ReactNode }) {
  return <div className="p-5">{children}</div>
}
