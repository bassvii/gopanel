import { useState, type ReactNode } from 'react'

interface Props {
  content: string
  children: ReactNode
}

export function Tooltip({ content, children }: Props) {
  const [visible, setVisible] = useState(false)

  return (
    <div
      className="relative inline-block"
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
    >
      {children}
      {visible && (
        <div
          className="absolute z-50 left-1/2 -translate-x-1/2 bottom-full mb-2 px-3 py-2 rounded-lg text-xs whitespace-nowrap pointer-events-none"
          style={{
            background: '#1a1a1a',
            border: '1px solid var(--border-strong)',
            color: 'var(--text)',
            boxShadow: '0 4px 16px rgba(0, 0, 0, 0.5)',
          }}
        >
          {content}
          <div
            className="absolute left-1/2 -translate-x-1/2 top-full w-0 h-0"
            style={{
              borderLeft: '5px solid transparent',
              borderRight: '5px solid transparent',
              borderTop: '5px solid var(--border-strong)',
            }}
          />
        </div>
      )}
    </div>
  )
}
