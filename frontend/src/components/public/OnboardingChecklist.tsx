type ChecklistItem = {
  title: string
  detail: string
}

const items: ChecklistItem[] = [
  {
    title: 'Create billing plan',
    detail: 'POST /admin/v1/billing-plans and assign the plan to a tenant.',
  },
  {
    title: 'Create tenant',
    detail: 'Create tenant, then set active tenant in Admin > Tenants.',
  },
  {
    title: 'Configure API product',
    detail: 'Create API product for routing and policy binding.',
  },
  {
    title: 'Create upstream and route',
    detail: 'Define upstream first, then create route with protocol mapping.',
  },
  {
    title: 'Create consumer credential',
    detail: 'Issue API key credential and test request with gateway endpoint.',
  },
]

export function OnboardingChecklist() {
  return (
    <section className="panel">
      <h3>Operator Onboarding Checklist</h3>
      <ol className="checklist">
        {items.map((item) => (
          <li key={item.title}>
            <strong>{item.title}</strong>
            <p>{item.detail}</p>
          </li>
        ))}
      </ol>
    </section>
  )
}
