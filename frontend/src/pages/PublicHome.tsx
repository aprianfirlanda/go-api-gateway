import { OnboardingChecklist } from '../components/public/OnboardingChecklist'
import { SystemStatus } from '../components/public/SystemStatus'

export function PublicHome() {
  return (
    <section>
      <h1>Gateway Operations Hub</h1>
      <p>
        This page provides operational checks and setup guidance for running the gateway and
        control-plane services.
      </p>
      <div className="grid-2">
        <SystemStatus />
        <OnboardingChecklist />
      </div>
    </section>
  )
}
