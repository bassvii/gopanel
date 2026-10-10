// SPDX-License-Identifier: AGPL-3.0-or-later

import { Modal } from './Modal'

interface Props {
  title: string
  url: string
  qrBase64: string
  onClose: () => void
}

export function QRModal({ title, url, qrBase64, onClose }: Props) {
  function download() {
    const link = document.createElement('a')
    link.href = `data:image/png;base64,${qrBase64}`
    link.download = `${title.replace(/[^\w-]+/g, '_')}.png`
    link.click()
  }

  const clean = qrBase64.replace(/^data:image\/png;base64,/, '')

  return (
    <Modal title={title} onClose={onClose}>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
        <div style={{ background: 'white', padding: 12, borderRadius: 8 }}>
          <img
            src={`data:image/png;base64,${clean}`}
            alt="QR"
            width={256}
            height={256}
            style={{
              display: 'block',
              width: 256,
              height: 256,
              imageRendering: 'pixelated',
            }}
          />
        </div>

        <div
          style={{
            fontSize: 11,
            color: '#888',
            wordBreak: 'break-all',
            fontFamily: 'monospace',
            textAlign: 'center',
            maxWidth: 400,
          }}
        >
          {url}
        </div>

        <div style={{ display: 'flex', gap: 8 }}>
          <button
            onClick={() => navigator.clipboard.writeText(url)}
            style={{
              background: '#262626',
              color: 'white',
              border: 'none',
              borderRadius: 4,
              padding: '8px 16px',
              cursor: 'pointer',
              fontSize: 14,
            }}
          >
            Копировать ссылку
          </button>
          <button
            onClick={download}
            style={{
              background: 'white',
              color: 'black',
              border: 'none',
              borderRadius: 4,
              padding: '8px 16px',
              cursor: 'pointer',
              fontSize: 14,
              fontWeight: 500,
            }}
          >
            Скачать PNG
          </button>
        </div>
      </div>
    </Modal>
  )
}
