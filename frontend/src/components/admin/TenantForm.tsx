import { useState } from 'react'
import { Form } from '../ui/Form'

type TenantFormProps = {
  onCreate: (payload: { name: string; slug: string }) => Promise<void>
}

export function TenantForm({ onCreate }: TenantFormProps) {
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [submitting, setSubmitting] = useState(false)

  return (
    <Form
      title="Create Tenant"
      onSubmit={async (event) => {
        event.preventDefault()
        setSubmitting(true)
        try {
          await onCreate({ name, slug })
          setName('')
          setSlug('')
        } finally {
          setSubmitting(false)
        }
      }}
      actions={
        <button type="submit" disabled={submitting || !name || !slug}>
          {submitting ? 'Creating...' : 'Create'}
        </button>
      }
    >
      <label>
        Name
        <input value={name} onChange={(event) => setName(event.target.value)} />
      </label>
      <label>
        Slug
        <input value={slug} onChange={(event) => setSlug(event.target.value)} />
      </label>
    </Form>
  )
}
