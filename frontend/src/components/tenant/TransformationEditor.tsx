import { useState } from 'react'
import { Form } from '../ui/Form'

type TransformationEditorProps = {
  onCreate: (payload: {
    apiProductId: string
    name: string
    sourceProtocol: string
    targetProtocol: string
    templateBody: Record<string, unknown>
  }) => Promise<void>
}

export function TransformationEditor({ onCreate }: TransformationEditorProps) {
  const [apiProductId, setApiProductId] = useState('')
  const [name, setName] = useState('')
  const [sourceProtocol, setSourceProtocol] = useState('rest')
  const [targetProtocol, setTargetProtocol] = useState('iso8583')
  const [templateBody, setTemplateBody] = useState('{"map":{}}')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  return (
    <Form
      title="Create Transformation Template"
      onSubmit={async (event) => {
        event.preventDefault()
        setError('')
        setSubmitting(true)
        try {
          await onCreate({
            apiProductId,
            name,
            sourceProtocol,
            targetProtocol,
            templateBody: JSON.parse(templateBody) as Record<string, unknown>,
          })
          setApiProductId('')
          setName('')
          setTemplateBody('{"map":{}}')
        } catch (err) {
          setError(err instanceof Error ? err.message : 'Failed to create template')
        } finally {
          setSubmitting(false)
        }
      }}
      actions={
        <button type="submit" disabled={submitting || !apiProductId || !name}>
          {submitting ? 'Creating...' : 'Create Template'}
        </button>
      }
    >
      <label>
        API Product ID
        <input value={apiProductId} onChange={(event) => setApiProductId(event.target.value)} />
      </label>
      <label>
        Name
        <input value={name} onChange={(event) => setName(event.target.value)} />
      </label>
      <label>
        Source Protocol
        <input value={sourceProtocol} onChange={(event) => setSourceProtocol(event.target.value)} />
      </label>
      <label>
        Target Protocol
        <input value={targetProtocol} onChange={(event) => setTargetProtocol(event.target.value)} />
      </label>
      <label>
        Template Body (JSON)
        <textarea
          value={templateBody}
          onChange={(event) => setTemplateBody(event.target.value)}
          rows={8}
        />
      </label>
      {error ? <p>{error}</p> : null}
    </Form>
  )
}
