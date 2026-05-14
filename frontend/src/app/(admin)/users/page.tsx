import { useEffect, useState } from 'react'
import { apiRequest } from '../../../lib/api/client'

export function AdminUsersPage() {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    const load = async () => {
      setLoading(true)
      setError('')
      try {
        await apiRequest('/admin/v1/users', {
          headers: { Authorization: 'Bearer dev-admin-token' },
        })
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load users')
      } finally {
        setLoading(false)
      }
    }
    void load()
  }, [])

  return (
    <section>
      <h2>Users and Roles</h2>
      {loading ? <p>Loading users...</p> : null}
      {error ? (
        <p>
          Backend user management endpoint is not available yet: <code>{error}</code>
        </p>
      ) : (
        <p>Users loaded.</p>
      )}
    </section>
  )
}
