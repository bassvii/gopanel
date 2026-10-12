interface Props {
  enabled: boolean
  onChange: () => void
  disabled?: boolean
}

export function Toggle({ enabled, onChange, disabled }: Props) {
  return (
    <button
      type="button"
      onClick={onChange}
      disabled={disabled}
      className="relative inline-flex items-center rounded-full transition-colors"
      style={{
        width: 36,
        height: 20,
        background: enabled ? 'var(--accent)' : '#333333',
        cursor: disabled ? 'not-allowed' : 'pointer',
        opacity: disabled ? 0.5 : 1,
      }}
      aria-pressed={enabled}
    >
      <span
        className="rounded-full bg-white transition-transform"
        style={{
          width: 14,
          height: 14,
          transform: `translateX(${enabled ? 18 : 3}px)`,
        }}
      />
    </button>
  )
}
