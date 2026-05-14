export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export type ApiRequestOptions = {
  method?: HttpMethod
  headers?: Record<string, string>
  body?: unknown
  signal?: AbortSignal
}

export type ApiListResponse<T> = {
  data: T[]
  nextCursor?: string | null
}

export type Tenant = {
  id: string
  name: string
  slug: string
  status: string
  billingPlanId?: string
  createdAt: string
  updatedAt: string
}

export type APIProduct = {
  id: string
  tenantId: string
  name: string
  slug: string
  status: string
}

export type Upstream = {
  id: string
  tenantId: string
  name: string
  protocol: string
  status: string
}

export type Route = {
  id: string
  tenantId: string
  apiProductId: string
  name: string
  inboundProtocol: string
  outboundProtocol: string
  host: string
  method: string
  path: string
  upstreamId: string
  status: string
}

export type Consumer = {
  id: string
  tenantId: string
  name: string
  slug: string
  status: string
}

export type CredentialCreateResponse = {
  id: string
  type: string
  keyPrefix: string
  apiKey?: string
  status: string
}

export type BillingSummary = {
  tenantId: string
  billingPeriod: string
  totalRequests: number
  billableRequests: number
  overageRequests: number
  estimatedAmount: number
  currency: string
  status: string
}

export type AuditLog = {
  id: string
  actorId: string
  tenantId?: string
  action: string
  resource: string
  resourceId: string
  occurredAt: string
}

export type TransformationTemplate = {
  id: string
  tenantId: string
  apiProductId: string
  name: string
  sourceProtocol: string
  targetProtocol: string
  status: string
  version: number
}
