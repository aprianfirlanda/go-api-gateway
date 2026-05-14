import { Table } from '../ui/Table'
import type { CredentialCreateResponse } from '../../lib/api/types'

type CredentialTableProps = {
  rows: CredentialCreateResponse[]
  onRotate: (credentialId: string) => void
}

export function CredentialTable({ rows, onRotate }: CredentialTableProps) {
  return (
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
            <button type="button" onClick={() => onRotate(row.id)}>
              Rotate
            </button>
          ),
        },
      ]}
      rows={rows}
      rowKey={(row) => row.id}
    />
  )
}
