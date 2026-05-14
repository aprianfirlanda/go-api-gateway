import type { ReactNode } from 'react'

type ModalProps = {
  title: string
  open: boolean
  onClose: () => void
  children: ReactNode
}

export function Modal({ title, open, onClose, children }: ModalProps) {
  if (!open) {
    return null
  }

  return (
    <div className="ui-modal-backdrop" role="presentation" onClick={onClose}>
      <section
        className="ui-modal"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(event) => event.stopPropagation()}
      >
        <header className="ui-modal-header">
          <h3>{title}</h3>
          <button type="button" className="icon-btn" aria-label="Close modal" onClick={onClose}>
            ×
          </button>
        </header>
        <div className="ui-modal-body">{children}</div>
      </section>
    </div>
  )
}
