const ACTIVE_TENANT_KEY = 'gateway_active_tenant'

export function getActiveTenantId(): string {
  if (typeof window === 'undefined') {
    return ''
  }
  return window.localStorage.getItem(ACTIVE_TENANT_KEY) ?? ''
}

export function setActiveTenantId(tenantId: string): void {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(ACTIVE_TENANT_KEY, tenantId)
  }
}
