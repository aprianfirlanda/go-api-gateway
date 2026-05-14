import { SystemStatus } from '../components/public/SystemStatus'

export function PublicStatus() {
  return (
    <section>
      <h2>Service Status</h2>
      <p>Run health and readiness checks for gateway and control plane.</p>
      <SystemStatus />
    </section>
  )
}
