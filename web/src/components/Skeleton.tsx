interface SkeletonProps {
  className?: string
  width?: string | number
  height?: string | number
}

export function Skeleton({ className = '', width, height }: SkeletonProps) {
  return (
    <div
      className={`animate-pulse rounded ${className}`}
      style={{
        background: '#1a1a1a',
        width,
        height,
      }}
    />
  )
}

// TableSkeleton — скелетон таблицы (5 строк).
export function TableSkeleton({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="border border-neutral-800 rounded-lg overflow-hidden">
      <div
        className="px-4 py-3 flex gap-4"
        style={{ background: '#0a0a0a', borderBottom: '1px solid var(--border)' }}
      >
        {Array.from({ length: cols }).map((_, i) => (
          <Skeleton key={i} width="80px" height="12px" />
        ))}
      </div>
      {Array.from({ length: rows }).map((_, r) => (
        <div
          key={r}
          className="px-4 py-3 flex gap-4"
          style={{ borderBottom: r < rows - 1 ? '1px solid var(--border)' : 'none' }}
        >
          {Array.from({ length: cols }).map((_, c) => (
            <Skeleton
              key={c}
              width={c === 0 ? '120px' : c === cols - 1 ? '60px' : '90px'}
              height="14px"
            />
          ))}
        </div>
      ))}
    </div>
  )
}

// CardSkeleton — скелетон карточки.
export function CardSkeleton() {
  return (
    <div className="card p-5 space-y-3">
      <Skeleton width="80px" height="12px" />
      <Skeleton width="60px" height="28px" />
      <Skeleton width="100px" height="12px" />
    </div>
  )
}
