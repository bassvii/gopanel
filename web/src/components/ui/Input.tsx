import { forwardRef, type InputHTMLAttributes, type ReactNode } from 'react'

interface Props extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  hint?: string
  error?: string
  suffix?: ReactNode
}

export const Input = forwardRef<HTMLInputElement, Props>(function Input(
  { label, hint, error, suffix, className = '', ...rest },
  ref,
) {
  return (
    <label className="block">
      {label && (
        <span className="text-sm text-neutral-400 block mb-1">{label}</span>
      )}
      <div className="flex gap-2">
        <input
          ref={ref}
          className={
            'w-full rounded-lg px-3 py-2 text-white text-sm outline-none transition-colors ' +
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
