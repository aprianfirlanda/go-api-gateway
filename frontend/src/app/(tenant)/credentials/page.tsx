import { useState } from 'react'
import { CredentialRotateDialog } from '../../../components/admin/CredentialRotateDialog'
import { CredentialTable } from '../../../components/tenant/CredentialTable'
import { EmptyState } from '../../../components/ui/EmptyState'
import { apiRequest } from '../../../lib/api/client'
import { controlPlaneAuthHeaders, getAuthSession } from '../../../lib/auth'
import type { Consumer, CredentialCreateResponse } from '../../../lib/api/types'

export function TenantCredentialsPage() {
  const session = getAuthSession()
  const [consumer, setConsumer] = useState<Consumer | null>(null)
  const [consumerName, setConsumerName] = useState('')
  const [consumerSlug, setConsumerSlug] = useState('')
  const [credentials, setCredentials] = useState<CredentialCreateResponse[]>([])
  const [rotatingCredentialId, setRotatingCredentialId] = useState('')
  const [error, setError] = useState('')

  if (session.role !== 'tenant_admin' || !session.tenantId) {
    return (
      <EmptyState
        title="Unauthorized"
        description="Tenant admin session is required for credential access."
      />
    )
  }

  return (
    <section>
      <h2>Tenant Credentials</h2>
      <p>Tenant ID: {session.tenantId}</p>

      <form
        className="ui-form"
        onSubmit={async (event) => {
          event.preventDefault()
          setError('')
          try {
            const created = await apiRequest<Consumer>(`/admin/v1/tenants/${session.tenantId}/consumers`, {
              method: 'POST',
              headers: controlPlaneAuthHeaders(session),
              body: { name: consumerName, slug: consumerSlug },
            })
            setConsumer(created)
          } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to create consumer')
          }
        }}
      >
        <h3>Create Consumer</h3>
        <label>
          Name
          <input value={consumerName} onChange={(event) => setConsumerName(event.target.value)} />
        </label>
        <label>
          Slug
          <input value={consumerSlug} onChange={(event) => setConsumerSlug(event.target.value)} />
        </label>
        <div className="ui-form-actions">
          <button type="submit" disabled={!consumerName || !consumerSlug}>
            Create Consumer
          </button>
        </div>
      </form>

      {consumer ? (
        <form
          className="ui-form"
          onSubmit={async (event) => {
            event.preventDefault()
            setError('')
            try {
              const created = await apiRequest<CredentialCreateResponse>(
                `/admin/v1/tenants/${session.tenantId}/consumers/${consumer.id}/credentials`,
                {
                  method: 'POST',
                  headers: controlPlaneAuthHeaders(session),
                  body: { type: 'api_key' },
                },
              )
              setCredentials((prev) => [created, ...prev])
            } catch (err) {
              setError(err instanceof Error ? err.message : 'Failed to create credential')
            }
          }}
        >
          <h3>Create Credential</h3>
          <p>Consumer ID: {consumer.id}</p>
          <div className="ui-form-actions">
            <button type="submit">Issue API Key</button>
          </div>
        </form>
      ) : null}

      {error ? <p>{error}</p> : null}
      {credentials.length === 0 ? (
        <EmptyState title="No credentials" description="Create consumer and issue API key." />
      ) : (
        <CredentialTable rows={credentials} onRotate={(id) => setRotatingCredentialId(id)} />
      )}

      <CredentialRotateDialog
        open={!!rotatingCredentialId}
        credentialId={rotatingCredentialId}
        onClose={() => setRotatingCredentialId('')}
        onRotate={(credentialId) =>
          apiRequest<CredentialCreateResponse>(
            `/admin/v1/tenants/${session.tenantId}/credentials/${credentialId}/rotate`,
            {
              method: 'POST',
              headers: controlPlaneAuthHeaders(session),
            },
          )
        }
      />
    </section>
  )
}
