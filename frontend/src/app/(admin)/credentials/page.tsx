import { useState } from 'react'
import { CredentialRotateDialog } from '../../../components/admin/CredentialRotateDialog'
import { EmptyState } from '../../../components/ui/EmptyState'
import { Table } from '../../../components/ui/Table'
import { apiRequest } from '../../../lib/api/client'
import type { Consumer, CredentialCreateResponse } from '../../../lib/api/types'
import { getActiveTenantId } from '../../../lib/adminContext'

export function AdminCredentialsPage() {
  const tenantId = getActiveTenantId()
  const [consumerName, setConsumerName] = useState('')
  const [consumerSlug, setConsumerSlug] = useState('')
  const [consumer, setConsumer] = useState<Consumer | null>(null)
  const [credentials, setCredentials] = useState<CredentialCreateResponse[]>([])
  const [error, setError] = useState('')
  const [rotatingCredentialId, setRotatingCredentialId] = useState('')

  if (!tenantId) {
    return <EmptyState title="No tenant selected" description="Go to Tenants and set an active tenant first." />
  }

  return (
    <section>
      <h2>Credentials</h2>
      <p>Active tenant: {tenantId}</p>

      <form
        className="ui-form"
        onSubmit={async (event) => {
          event.preventDefault()
          setError('')
          try {
            const created = await apiRequest<Consumer>(`/admin/v1/tenants/${tenantId}/consumers`, {
              method: 'POST',
              headers: { Authorization: 'Bearer dev-admin-token' },
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
                `/admin/v1/tenants/${tenantId}/consumers/${consumer.id}/credentials`,
                {
                  method: 'POST',
                  headers: { Authorization: 'Bearer dev-admin-token' },
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
        <EmptyState title="No credentials issued" description="Create a consumer and issue an API key." />
      ) : (
        <Table
          columns={[
            { key: 'id', header: 'Credential ID' },
            { key: 'keyPrefix', header: 'Key Prefix' },
            { key: 'status', header: 'Status' },
            {
              key: 'apiKey',
              header: 'API Key',
              render: (row) => (row.apiKey ? <code>{row.apiKey}</code> : '-'),
            },
            {
              key: 'actions',
              header: 'Actions',
              render: (row) => (
                <button type="button" onClick={() => setRotatingCredentialId(row.id)}>
                  Rotate
                </button>
              ),
            },
          ]}
          rows={credentials}
          rowKey={(row) => row.id}
        />
      )}

      <CredentialRotateDialog
        open={!!rotatingCredentialId}
        credentialId={rotatingCredentialId}
        onClose={() => setRotatingCredentialId('')}
        onRotate={async (credentialId) =>
          apiRequest<CredentialCreateResponse>(
            `/admin/v1/tenants/${tenantId}/credentials/${credentialId}/rotate`,
            {
              method: 'POST',
              headers: { Authorization: 'Bearer dev-admin-token' },
            },
          )
        }
      />
    </section>
  )
}
