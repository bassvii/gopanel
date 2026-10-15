// SPDX-License-Identifier: AGPL-3.0-or-later

import { AlertCircle, RefreshCw } from 'lucide-react'

interface Props {
  title?: string
  message: string
  onRetry?: () => void
}

export function ErrorBanner({ title, message, onRetry }: Props) {
  return (
    <div
      className="rounded-lg p-4 flex items-start gap-3"
      style={{
        background: 'rgba(239, 68, 68, 0.08)',
        border: '1px solid rgba(239, 68, 68, 0.25)',
      }}
    >
      <AlertCircle
        size={20}
        className="shrink-0 mt-0.5"
        style={{ color: 'var(--danger)' }}
      />
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium text-white mb-0.5">
          {title ?? 'Что-то пошло не так'}
        </div>
        <div className="text-sm break-words" style={{ color: 'var(--text-muted)' }}>
          {message}
        </div>
        {onRetry && (
          <button
            onClick={onRetry}
            className="inline-flex items-center gap-1.5 text-xs mt-2 transition-colors hover:opacity-80"
            style={{ color: 'var(--accent)' }}
          >
            <RefreshCw size={12} />
            Попробовать снова
          </button>
        )}
      </div>
    </div>
  )
}
