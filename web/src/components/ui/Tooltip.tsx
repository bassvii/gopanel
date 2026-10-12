import { useState, type ReactNode } from 'react'

interface Props {
  content: string
  children: ReactNode
  align?: 'center' | 'right'
}

export function Tooltip({ content, children, align = 'center' }: Props) {
  const [visible, setVisible] = useState(false)

  const positionStyle: React.CSSProperties =
    align === 'right'
      ? { right: 0 }
      : { left: '50%', transform: 'translateX(-50%)' }

  const arrowStyle: React.CSSProperties =
    align === 'right'
      ? { right: 12 }
      : { left: '50%', transform: 'translateX(-50%)' }

  return (
    <div
      className="relative inline-block"
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
    >
      {children}
      {visible && (
        <div
          className="absolute z-50 bottom-full mb-2 px-3 py-2 rounded-lg text-xs whitespace-nowrap pointer-events-none"
          style={{
            background: '#1a1a1a',
            border: '1px solid var(--border-strong)',
            color: 'var(--text)',
            boxShadow: '0 4px 16px rgba(0, 0, 0, 0.5)',
            ...positionStyle,
          }}
        >
          {content}
          <div
            className="absolute top-full w-0 h-0"
            style={{
              borderLeft: '5px solid transparent',
              borderRight: '5px solid transparent',
              borderTop: '5px solid var(--border-strong)',
              ...arrowStyle,
            }}
          />
        </div>
      )}
    </div>
  )
}
