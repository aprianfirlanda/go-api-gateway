import type { AuthSession, Role } from './auth'

const rolePriority: Record<Role, number> = {
  public: 0,
  tenant_admin: 1,
  platform_admin: 2,
}

export function canAccessRole(
  session: AuthSession | null,
  requiredRole: Exclude<Role, 'public'>,
): boolean {
  if (!session) {
    return false
  }
  return rolePriority[session.role] >= rolePriority[requiredRole]
}
