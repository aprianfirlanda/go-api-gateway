import { useState } from 'react'
import { Form } from '../ui/Form'

type RouteFormPayload = {
  apiProductId: string
  name: string
  inboundProtocol: string
  outboundProtocol: string
  host: string
  method: string
  path: string
  upstreamId: string
}

type RouteFormProps = {
  onCreate: (payload: RouteFormPayload) => Promise<void>
}

export function RouteForm({ onCreate }: RouteFormProps) {
  const [form, setForm] = useState<RouteFormPayload>({
    apiProductId: '',
    name: '',
    inboundProtocol: 'rest',
    outboundProtocol: 'rest',
    host: '',
    method: 'GET',
    path: '',
    upstreamId: '',
  })
  const [submitting, setSubmitting] = useState(false)

  const update = (key: keyof RouteFormPayload, value: string) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  return (
    <Form
      title="Create Route"
      onSubmit={async (event) => {
        event.preventDefault()
        setSubmitting(true)
        try {
          await onCreate(form)
          setForm({
            apiProductId: '',
            name: '',
            inboundProtocol: 'rest',
            outboundProtocol: 'rest',
            host: '',
            method: 'GET',
            path: '',
            upstreamId: '',
          })
        } finally {
          setSubmitting(false)
        }
      }}
      actions={
        <button
          type="submit"
          disabled={submitting || !form.name || !form.path || !form.apiProductId || !form.upstreamId || !form.host}
        >
          {submitting ? 'Creating...' : 'Create Route'}
        </button>
      }
    >
      <label>
        API Product ID
        <input value={form.apiProductId} onChange={(event) => update('apiProductId', event.target.value)} />
      </label>
      <label>
        Name
        <input value={form.name} onChange={(event) => update('name', event.target.value)} />
      </label>
      <label>
        Inbound Protocol
        <input value={form.inboundProtocol} onChange={(event) => update('inboundProtocol', event.target.value)} />
      </label>
      <label>
        Outbound Protocol
        <input value={form.outboundProtocol} onChange={(event) => update('outboundProtocol', event.target.value)} />
      </label>
      <label>
        Host
        <input value={form.host} onChange={(event) => update('host', event.target.value)} />
      </label>
      <label>
        Method
        <input value={form.method} onChange={(event) => update('method', event.target.value)} />
      </label>
      <label>
        Path
        <input value={form.path} onChange={(event) => update('path', event.target.value)} />
      </label>
      <label>
        Upstream ID
        <input value={form.upstreamId} onChange={(event) => update('upstreamId', event.target.value)} />
      </label>
    </Form>
  )
}
