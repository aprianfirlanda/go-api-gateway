export type ServiceStatus = {
  service: 'gateway' | 'control-plane'
  health: 'healthy' | 'unhealthy'
  ready: 'ready' | 'not_ready'
  checkedAt: string
  error?: string
}

async function fetchPath(url: string): Promise<void> {
  const response = await fetch(url, { method: 'GET' })
  if (!response.ok) {
    throw new Error(`status ${response.status}`)
  }
}

async function check(baseUrl: string, service: 'gateway' | 'control-plane'): Promise<ServiceStatus> {
  const checkedAt = new Date().toISOString()

  try {
    await fetchPath(`${baseUrl}/healthz`)
    let ready: ServiceStatus['ready']
    try {
      await fetchPath(`${baseUrl}/readyz`)
      ready = 'ready'
    } catch {
      ready = 'not_ready'
    }
    return { service, health: 'healthy', ready, checkedAt }
  } catch (err) {
    return {
      service,
      health: 'unhealthy',
      ready: 'not_ready',
      checkedAt,
      error: err instanceof Error ? err.message : 'status check failed',
    }
  }
}

export async function fetchSystemStatus(): Promise<ServiceStatus[]> {
  const gatewayUrl = import.meta.env.VITE_GATEWAY_BASE_URL ?? 'http://localhost:8080'
  const controlPlaneUrl = import.meta.env.VITE_CONTROL_PLANE_BASE_URL ?? 'http://localhost:8081'

  const [gateway, controlPlane] = await Promise.all([
    check(gatewayUrl, 'gateway'),
    check(controlPlaneUrl, 'control-plane'),
  ])

  return [gateway, controlPlane]
}
