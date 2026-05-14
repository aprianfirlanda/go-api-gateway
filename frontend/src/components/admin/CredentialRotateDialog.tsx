import { useState } from 'react'
import { Modal } from '../ui/Modal'
import type { CredentialCreateResponse } from '../../lib/api/types'

type CredentialRotateDialogProps = {
  open: boolean
  credentialId: string
  onClose: () => void
  onRotate: (credentialId: string) => Promise<CredentialCreateResponse>
}

export function CredentialRotateDialog({
  open,
  credentialId,
  onClose,
  onRotate,
}: CredentialRotateDialogProps) {
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<CredentialCreateResponse | null>(null)
  const [error, setError] = useState('')

  const rotate = async () => {
    setSubmitting(true)
    setError('')
    try {
      const data = await onRotate(credentialId)
      setResult(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Rotate failed')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal title="Rotate Credential" open={open} onClose={onClose}>
      <p>Credential ID: {credentialId}</p>
      <button type="button" onClick={rotate} disabled={submitting || !credentialId}>
        {submitting ? 'Rotating...' : 'Rotate'}
      </button>
      {error ? <p>{error}</p> : null}
      {result?.apiKey ? (
        <p>
          New API key: <code>{result.apiKey}</code>
        </p>
      ) : null}
    </Modal>
  )
}
