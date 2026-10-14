import { forwardRef, useState, type InputHTMLAttributes, type ReactNode } from 'react'
import { Eye, EyeOff } from 'lucide-react'

interface Props extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  hint?: string
  error?: string
  suffix?: ReactNode
}

export const Input = forwardRef<HTMLInputElement, Props>(function Input(
  { label, hint, error, suffix, className = '', type = 'text', ...rest },
  ref,
) {
  const [showPassword, setShowPassword] = useState(false)
  const isPassword = type === 'password'
  const effectiveType = isPassword && showPassword ? 'text' : type

  return (
    <label className="block">
      {label && (
        <span className="text-sm text-neutral-400 block mb-1">{label}</span>
      )}
      <div className="flex gap-2 relative">
        <input
          ref={ref}
          type={effectiveType}
          className={
            'w-full rounded-lg px-3 py-2 text-white text-sm outline-none transition-colors ' +
            (isPassword ? 'pr-10 ' : '') +
            className
          }
          style={{
            background: '#0a0a0a',
            border: `1px solid ${error ? 'var(--danger)' : 'var(--border)'}`,
          }}
          onFocus={(e) => {
            e.currentTarget.style.borderColor = error ? 'var(--danger)' : 'var(--accent)'
            rest.onFocus?.(e)
          }}
          onBlur={(e) => {
            e.currentTarget.style.borderColor = error ? 'var(--danger)' : 'var(--border)'
            rest.onBlur?.(e)
          }}
          {...rest}
        />
        {isPassword && (
          <button
            type="button"
            onClick={() => setShowPassword((v) => !v)}
            disabled={rest.disabled}
            aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
            tabIndex={-1}
            className="absolute right-3 top-1/2 -translate-y-1/2 flex items-center justify-center text-neutral-500 hover:text-neutral-300 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              padding: 4,
              lineHeight: 0,
            }}
          >
            {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
        )}
        {suffix}
      </div>
      {hint && !error && (
        <span className="text-xs text-neutral-500 block mt-1">{hint}</span>
      )}
      {error && (
        <span className="text-xs block mt-1" style={{ color: 'var(--danger)' }}>
          {error}
        </span>
      )}
    </label>
  )
})
