import { AlertTriangle } from 'lucide-react'

interface Props {
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmDialog({
  title,
  message,
  confirmLabel = 'Подтвердить',
  cancelLabel = 'Отмена',
  danger,
  onConfirm,
  onCancel,
}: Props) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0, 0, 0, 0.6)' }}
      onClick={onCancel}
    >
      <div
        className="w-full max-w-sm rounded-xl"
        style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--border)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="p-5 flex gap-3">
          {danger && (
            <div
              className="w-10 h-10 rounded-full flex items-center justify-center shrink-0"
              style={{ background: 'rgba(239, 68, 68, 0.1)' }}
            >
              <AlertTriangle size={20} style={{ color: 'var(--danger)' }} />
            </div>
          )}
          <div className="flex-1 min-w-0">
            <h3 className="text-base font-semibold text-white mb-1">{title}</h3>
            <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {message}
            </p>
          </div>
        </div>

        <div
          className="px-5 py-3 flex justify-end gap-2"
          style={{ borderTop: '1px solid var(--border)' }}
        >
          <button onClick={onCancel} className="btn btn-ghost">
            {cancelLabel}
          </button>
          <button
            onClick={onConfirm}
            className={danger ? 'btn btn-danger' : 'btn btn-primary'}
            autoFocus
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
