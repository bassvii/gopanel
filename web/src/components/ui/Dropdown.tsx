import { useEffect, useRef, useState, type ReactNode } from 'react'

interface Props {
  trigger: (open: boolean) => ReactNode
  children: ReactNode
  align?: 'left' | 'right'
  width?: number
}

export function Dropdown({ trigger, children, align = 'right', width = 320 }: Props) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    function onClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    function onEsc(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onClickOutside)
    document.addEventListener('keydown', onEsc)
    return () => {
      document.removeEventListener('mousedown', onClickOutside)
      document.removeEventListener('keydown', onEsc)
    }
  }, [open])

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="outline-none"
      >
        {trigger(open)}
      </button>

      {open && (
        <div
          className="absolute top-full mt-2 z-50 rounded-xl overflow-hidden"
          style={{
            [align]: 0,
            width,
            background: 'var(--bg-surface)',
            border: '1px solid var(--border)',
            boxShadow: '0 10px 38px -10px rgba(0, 0, 0, 0.6)',
          }}
        >
          {children}
        </div>
      )}
    </div>
  )
}
