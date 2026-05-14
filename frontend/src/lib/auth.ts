export type Role = 'public' | 'platform_admin' | 'tenant_admin'

export type AuthSession = {
  userId: string
  tenantId?: string
  role: Role
}

const SESSION_KEY = 'gateway_console_role'

export function getAuthSession(): AuthSession {
  const fromStorage =
    typeof window !== 'undefined' ? window.localStorage.getItem(SESSION_KEY) : null

  const role: Role =
    fromStorage === 'platform_admin' || fromStorage === 'tenant_admin'
      ? fromStorage
      : 'public'

  return {
    userId: 'local-dev-user',
    tenantId: role === 'tenant_admin' ? 'local-tenant-1' : undefined,
    role,
  }
}

export function setLocalRole(role: Role): void {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(SESSION_KEY, role)
  }
}

export function controlPlaneAuthHeaders(session: AuthSession): Record<string, string> {
  if (session.role === 'tenant_admin') {
    const tenantAdminApiKey = import.meta.env.VITE_TENANT_ADMIN_API_KEY
    if (tenantAdminApiKey) {
      return { 'X-Admin-Api-Key': tenantAdminApiKey }
    }
  }

  if (session.role === 'platform_admin') {
    const platformAdminApiKey = import.meta.env.VITE_PLATFORM_ADMIN_API_KEY
    if (platformAdminApiKey) {
      return { 'X-Admin-Api-Key': platformAdminApiKey }
    }
  }

  return { Authorization: 'Bearer dev-admin-token' }
}
