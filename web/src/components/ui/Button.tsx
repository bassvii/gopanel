import type { ButtonHTMLAttributes, ReactNode } from 'react'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  children: ReactNode
  icon?: ReactNode
}

const baseClasses =
  'inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap'

const variantClasses: Record<Variant, string> = {
  primary: 'text-white',
  secondary: 'text-white',
  ghost: '',
  danger: 'text-white',
}

const sizeClasses: Record<Size, string> = {
  sm: 'text-xs px-3 py-1.5',
  md: 'text-sm px-4 py-2',
}

// Inline-стили для цветов, которые нельзя выразить классами без ошибок.
function variantStyle(variant: Variant): React.CSSProperties {
  switch (variant) {
    case 'primary':
      return { background: 'var(--accent)' }
    case 'secondary':
      return { background: '#262626' }
    case 'ghost':
      return { background: 'transparent', color: 'var(--text-muted)' }
    case 'danger':
      return { background: 'var(--danger)' }
  }
}

export function Button({
  variant = 'primary',
  size = 'md',
  className = '',
  children,
  icon,
  ...rest
}: Props) {
  return (
    <button
      className={`${baseClasses} ${variantClasses[variant]} ${sizeClasses[size]} ${className}`}
      style={variantStyle(variant)}
      onMouseEnter={(e) => {
        if (variant === 'primary') e.currentTarget.style.background = 'var(--accent-hover)'
        if (variant === 'secondary') e.currentTarget.style.background = '#333333'
        if (variant === 'ghost') {
          e.currentTarget.style.background = '#1a1a1a'
          e.currentTarget.style.color = 'var(--text)'
        }
        if (variant === 'danger') e.currentTarget.style.background = '#dc2626'
        rest.onMouseEnter?.(e)
      }}
      onMouseLeave={(e) => {
        Object.assign(e.currentTarget.style, variantStyle(variant))
        rest.onMouseLeave?.(e)
      }}
      {...rest}
    >
      {icon}
      {children}
    </button>
  )
}
