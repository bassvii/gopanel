import { useState, type ReactNode } from 'react'
import { ConfirmDialog } from '../components/ConfirmDialog'

interface ConfirmOptions {
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
}

interface ConfirmState extends ConfirmOptions {
  resolve: (ok: boolean) => void
}

export function useConfirm() {
  const [state, setState] = useState<ConfirmState | null>(null)

  function confirm(opts: ConfirmOptions): Promise<boolean> {
    return new Promise((resolve) => {
      setState({ ...opts, resolve })
    })
  }

  function handleClose(ok: boolean) {
    if (state) {
      state.resolve(ok)
      setState(null)
    }
  }

  const dialog: ReactNode = state ? (
    <ConfirmDialog
      title={state.title}
      message={state.message}
      confirmLabel={state.confirmLabel}
      cancelLabel={state.cancelLabel}
      danger={state.danger}
      onConfirm={() => handleClose(true)}
      onCancel={() => handleClose(false)}
    />
  ) : null

  return { confirm, dialog }
}
