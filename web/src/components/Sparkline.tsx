import { useId } from 'react'

interface Props {
  data: number[]
  color?: string
  height?: number
  width?: number
  strokeWidth?: number
  fill?: boolean
  min?: number
  max?: number
  tension?: number
}

export function Sparkline({
  data,
  color = 'var(--accent)',
  height = 40,
  width = 120,
  strokeWidth = 1.5,
  fill = true,
  min,
  max,
  tension = 0.2,
}: Props) {
  const gradId = useId()

  if (!data || data.length < 2) {
    return <div style={{ height, width }} />
  }

  const dataMin = min ?? Math.min(...data)
  const dataMax = max ?? Math.max(...data)
  const range = dataMax - dataMin || 1

  const pad = strokeWidth
  const innerH = height - pad * 2
  const innerW = width - pad * 2
  const stepX = innerW / (data.length - 1)

  const points = data.map((v, i) => {
    const x = pad + i * stepX
    const y = pad + innerH - ((v - dataMin) / range) * innerH
    return [x, y] as const
  })

  const linePath = smoothPath(points, tension)

  const lastX = points[points.length - 1][0]
  const firstX = points[0][0]
  const baselineY = pad + innerH
  const areaPath = `${linePath} L ${lastX} ${baselineY} L ${firstX} ${baselineY} Z`

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      style={{ display: 'block', overflow: 'visible' }}
    >
      {fill && (
        <defs>
          <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity="0.35" />
            <stop offset="100%" stopColor={color} stopOpacity="0" />
          </linearGradient>
        </defs>
      )}

      {fill && <path d={areaPath} fill={`url(#${gradId})`} />}

      <path
        d={linePath}
        fill="none"
        stroke={color}
        strokeWidth={strokeWidth}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

// smoothPath строит сглаженную кривую через точки (Catmull-Rom → Bezier).
function smoothPath(points: readonly (readonly [number, number])[], tension = 0.2): string {
  if (points.length === 0) return ''
  if (points.length === 1) return `M ${points[0][0]} ${points[0][1]}`
  if (points.length === 2) {
    return `M ${points[0][0]} ${points[0][1]} L ${points[1][0]} ${points[1][1]}`
  }

  let d = `M ${points[0][0]} ${points[0][1]}`

  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[i - 1] ?? points[i]
    const p1 = points[i]
    const p2 = points[i + 1]
    const p3 = points[i + 2] ?? p2

    const cp1x = p1[0] + ((p2[0] - p0[0]) / 6) * tension * 6
    const cp1y = p1[1] + ((p2[1] - p0[1]) / 6) * tension * 6
    const cp2x = p2[0] - ((p3[0] - p1[0]) / 6) * tension * 6
    const cp2y = p2[1] - ((p3[1] - p1[1]) / 6) * tension * 6

    d += ` C ${cp1x} ${cp1y}, ${cp2x} ${cp2y}, ${p2[0]} ${p2[1]}`
  }

  return d
}
