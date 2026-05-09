# Product Design: Multitenant API Gateway for Finance Companies

## 1. Product Summary

The product is a multitenant API gateway built for finance companies that need to expose, consume, transform, secure, monitor, and monetize APIs across different protocols and integration standards.

The core MVP focuses on a protocol-adapter model for finance integrations. REST and ISO8583 are the first supported adapters because they solve a high-value payment integration use case, but the gateway should be designed to add more protocols such as SOAP/XML, gRPC, GraphQL, webhooks, file-based exchange, and message queues without redesigning the runtime.

The gateway should support multiple tenants, where each tenant can manage its own APIs, routing rules, transformation templates, security policies, usage limits, logs, and billing configuration without affecting other tenants.

Technical implementation details for building this gateway in Go are documented in [TECHNICAL_DESIGN.md](TECHNICAL_DESIGN.md).

## 2. Target Customers

Primary target customers:

- Banks
- Payment gateways
- Card issuers
- Acquirers
- Fintech companies
- Digital wallet providers
- Switching companies
- Financial SaaS providers

Primary users:

- API product managers
- Integration engineers
- Backend engineers
- DevOps and platform teams
- Security and compliance teams
- Finance and billing operations teams

## 3. Problem Statement

Finance companies often operate with a mix of modern APIs, legacy transaction systems, partner services, batch channels, and event-based integrations. Many financial systems still use ISO8583, SOAP/XML, fixed-width files, message queues, or proprietary TCP protocols, while partners and internal product teams increasingly expect REST, gRPC, GraphQL, or webhook-based APIs.

Common problems:

- Legacy and partner-specific protocols are difficult to expose safely to modern applications.
- Each partner integration often requires custom middleware.
- API security, rate limits, audit logs, and billing are commonly rebuilt per integration.
- Tenant isolation is hard when serving multiple business units, clients, or partner organizations.
- Finance teams need accurate usage-based billing for API consumption.
- Compliance teams need complete traceability of API calls and transaction transformations.

## 4. Product Goals

The product should:

- Provide a secure API gateway for finance-grade workloads.
- Transform requests and responses between supported protocols.
- Provide first-class support for REST and ISO8583 in the MVP.
- Provide an extensible adapter model for SOAP/XML, gRPC, GraphQL, webhooks, files, message queues, and proprietary TCP protocols.
- Support multiple tenants with strong data and configuration isolation.
- Allow each tenant to define APIs, routes, credentials, transformation rules, and billing plans.
- Provide metering, invoicing data, and billing reports.
- Provide observability for transactions, errors, latency, and usage.
- Reduce the time needed to launch partner and internal financial APIs.

## 5. Non-Goals for MVP

The MVP should not attempt to solve every integration pattern immediately.

Out of scope for MVP:

- Full API marketplace.
- Full GraphQL federation.
- Complete SOAP enterprise service bus replacement.
- Full file transfer management platform.
- General-purpose message broker.
- Native support for every ISO8583 variant.
- Real-time settlement.
- Payment orchestration engine.
- Fraud detection engine.
- End-user KYC or onboarding.
- Full accounting system.
- Self-service payment collection for invoices.

## 6. MVP Scope

The MVP should prove that finance companies can safely expose and consume transformed APIs with tenant isolation and billing visibility.

### 6.1 Tenant Management

MVP capabilities:

- Create and manage tenants.
- Assign tenant admin users.
- Store tenant-specific gateway configuration.
- Isolate tenant routes, credentials, logs, metrics, and billing records.
- Support tenant-level quotas and rate limits.

Tenant examples:

- A bank using one tenant for each business unit.
- A payment gateway using one tenant for each merchant group.
- A financial SaaS provider using one tenant for each client company.

### 6.2 API Gateway Routing and Protocol Adapters

MVP capabilities:

- Register APIs per tenant.
- Define public REST endpoints.
- Define protocol listeners for non-HTTP traffic.
- Route requests to backend REST services.
- Route requests to ISO8583 upstream systems through a connector.
- Route requests through a protocol adapter abstraction.
- Support path-based and method-based routing.
- Support listener-based routing for TCP-style protocols.
- Apply tenant-specific timeout, retry, and rate-limit policies.

Initial adapters:

- REST inbound and outbound.
- ISO8583 inbound and outbound.

MVP extensibility proof:

- REST to SOAP/XML outbound as a controlled proof adapter.

Planned adapter categories:

- Full SOAP/XML inbound and outbound.
- gRPC inbound and outbound.
- GraphQL inbound.
- Webhook inbound and outbound.
- Message queue inbound and outbound.
- File-based exchange, such as CSV, fixed-width, SFTP, and batch files.
- Proprietary TCP protocol adapters.

Example REST-to-ISO8583 route:

```http
POST /tenant-a/cards/authorization
```

Routes to:

```text
ISO8583 payment switch at 10.10.10.20:5000
```

### 6.3 ISO8583 to REST Transformation

The gateway should receive ISO8583 messages, parse selected fields, map them into JSON, and forward the result to a REST API.

Example ISO8583 authorization message fields:

```text
MTI: 0100
DE2: 411111******1111
DE3: 000000
DE4: 000000010000
DE7: 0508123015
DE11: 123456
DE37: 654321123456
DE41: ATM00101
DE49: 360
```

Transformed REST payload:

```json
{
  "messageType": "0100",
  "transactionType": "purchase",
  "panMasked": "411111******1111",
  "amount": 10000,
  "currency": "IDR",
  "transmissionDateTime": "0508123015",
  "stan": "123456",
  "rrn": "654321123456",
  "terminalId": "ATM00101"
}
```

### 6.4 REST to ISO8583 Transformation

The gateway should receive REST API calls, validate the payload, map the payload into ISO8583 fields, send the ISO8583 message to the upstream system, parse the response, and return a REST response.

Example REST request:

```json
{
  "transactionType": "purchase",
  "pan": "4111111111111111",
  "amount": 10000,
  "currency": "IDR",
  "terminalId": "ATM00101"
}
```

Generated ISO8583 message:

```text
MTI: 0100
DE2: 4111111111111111
DE3: 000000
DE4: 000000010000
DE7: generated_transmission_datetime
DE11: generated_stan
DE41: ATM00101
DE49: 360
```

Example REST response:

```json
{
  "status": "approved",
  "responseCode": "00",
  "stan": "123456",
  "rrn": "654321123456",
  "authorizationCode": "A12345"
}
```

### 6.5 Mult-Protocol Transformation

The gateway should not be limited to REST and ISO8583. It should use a canonical transformation model where each protocol adapter converts its native payload into an internal representation, then another adapter converts the internal representation into the target protocol.

Supported MVP transformation paths:

- REST to ISO8583.
- ISO8583 to REST.
- REST to REST.

Planned transformation paths:

- SOAP/XML to REST.
- REST to SOAP/XML.
- gRPC to REST.
- REST to gRPC.
- GraphQL to REST.
- REST to webhook.
- Message queue event to REST.
- File record to REST.
- Proprietary TCP message to REST or ISO8583.

Example canonical transaction object:

```json
{
  "transactionType": "purchase",
  "amount": 10000,
  "currency": "IDR",
  "accountRef": "masked_or_tokenized_reference",
  "terminalId": "ATM00101",
  "stan": "123456",
  "rrn": "654321123456",
  "metadata": {
    "sourceProtocol": "iso8583",
    "targetProtocol": "rest"
  }
}
```

### 6.6 Transformation Template Management

MVP capabilities:

- Define transformation templates per tenant and API.
- Support field mapping between protocol-specific fields and canonical fields.
- Support field mapping between JSON fields and ISO8583 data elements.
- Support XML path mapping for future SOAP/XML support.
- Support event and file record mapping for future adapters.
- Support simple value conversion, such as amount formatting and currency code mapping.
- Support masking rules for sensitive data.
- Version transformation templates.
- Roll back to a previous template version.

Example template concept:

```yaml
name: card-authorization-v1
direction: rest_to_iso8583
request:
  mti: "0100"
  fields:
    "2": "$.pan"
    "3": "000000"
    "4": "formatAmount($.amount)"
    "41": "$.terminalId"
    "49": "currencyNumeric($.currency)"
response:
  fields:
    responseCode: "39"
    authorizationCode: "38"
    stan: "11"
    rrn: "37"
```

### 6.7 Security

MVP capabilities:

- API key authentication.
- Tenant-level credential isolation.
- IP allowlist per tenant or API.
- TLS for all external HTTP APIs.
- Secure storage for credentials and secrets.
- Sensitive data masking in logs.

Planned security capabilities:

- OAuth2 client credentials support.
- HMAC request signing support for finance partners.
- mTLS for high-risk partners.

Sensitive data rules:

- Do not log full PAN.
- Do not log PIN blocks.
- Do not log CVV.
- Mask account numbers unless explicitly allowed by policy.

### 6.8 Rate Limiting and Quotas

MVP capabilities:

- Tenant-level rate limit.
- API-level rate limit.
- Consumer-level rate limit.
- Daily and monthly quota tracking.
- Configurable behavior when quota is exceeded.

Example:

```text
Tenant: Bank A
API: Card Authorization
Limit: 500 requests per second
Monthly quota: 50,000,000 requests
```

### 6.9 Billing

Billing should be usage-based and tenant-aware.

MVP capabilities:

- Meter every billable API request.
- Store usage by tenant, API, consumer, endpoint, and billing period.
- Support pricing plans.
- Generate billing reports.
- Export invoice-ready data.
- Support free quota, overage pricing, and fixed monthly fees.

Billing dimensions:

- Tenant
- API product
- Consumer application
- Request count
- Successful transaction count
- Failed transaction count
- Transformation type
- Source protocol
- Target protocol
- Billing period

Example pricing plans:

```text
Starter Plan
- Monthly fee: 0
- Included requests: 100,000
- Overage: 0.001 USD per request

Enterprise Plan
- Monthly fee: 5,000 USD
- Included requests: 10,000,000
- Overage: 0.0003 USD per request
- Dedicated support
```

Example billing record:

```json
{
  "tenantId": "tenant_bank_a",
  "apiId": "card_authorization",
  "consumerId": "mobile_app",
  "billingPeriod": "2026-05",
  "requestCount": 1250000,
  "successCount": 1242000,
  "failureCount": 8000,
  "includedQuota": 1000000,
  "billableOverage": 250000,
  "estimatedAmount": 75.0,
  "currency": "USD"
}
```

### 6.10 Observability and Audit

MVP capabilities:

- Request and response tracing.
- Transaction correlation ID.
- Tenant-level dashboards.
- API latency metrics.
- Error rate metrics.
- Transformation error logs.
- Protocol adapter error logs.
- Billing usage metrics.
- Audit logs for configuration changes.

Audit log examples:

- User created an API route.
- User updated a transformation template.
- User rotated an API key.
- User changed a billing plan.
- User updated a tenant quota.

### 6.11 Control Plane Admin Workflows

MVP capabilities:

- Tenant list.
- API list per tenant.
- Route configuration.
- Transformation template editor.
- Protocol adapter configuration.
- Credential management.
- Rate limit and quota configuration.
- Usage dashboard.
- Billing report page.
- Audit log page.

MVP delivery can be API-only. A browser-based admin portal can be added after the control plane APIs are stable.

### 6.12 Developer-Facing Workflows

MVP capabilities:

- API documentation per tenant.
- API credentials for consumer applications.
- Sample REST requests and responses.
- Protocol-specific integration examples.
- Usage summary.
- Error response reference.

## 7. Core User Journeys

### 7.1 Tenant Admin Creates a REST API for an ISO8583 Backend

1. Tenant admin creates an API named `Card Authorization`.
2. Tenant admin defines a REST endpoint.
3. Tenant admin configures the ISO8583 upstream host and port.
4. Tenant admin creates a REST-to-ISO8583 transformation template.
5. Tenant admin configures authentication and rate limits.
6. Tenant admin publishes the API.
7. A consumer calls the REST endpoint.
8. The gateway transforms the request to ISO8583.
9. The upstream switch responds with ISO8583.
10. The gateway transforms the response to JSON.
11. The gateway records usage for billing.

### 7.2 Tenant Admin Exposes ISO8583 Messages to a REST Backend

1. Tenant admin configures an ISO8583 listener.
2. Tenant admin maps ISO8583 fields to a REST payload.
3. Tenant admin configures the REST backend target.
4. External system sends an ISO8583 message.
5. Gateway parses and validates the message.
6. Gateway sends JSON payload to the REST backend.
7. Backend responds with JSON.
8. Gateway maps JSON response into ISO8583.
9. Gateway records audit, metrics, and billing usage.

### 7.3 Tenant Admin Publishes a SOAP/XML Service as REST

1. Tenant admin creates an API named `Customer Account Inquiry`.
2. Tenant admin configures a SOAP/XML upstream endpoint.
3. Tenant admin defines a REST endpoint for consumers.
4. Tenant admin maps JSON request fields into the SOAP request envelope.
5. Tenant admin maps SOAP response XML into JSON response fields.
6. Consumer calls the REST endpoint.
7. Gateway transforms the request, calls the SOAP service, transforms the response, and records billing usage.

### 7.4 Finance Team Reviews Billing

1. Finance user opens the billing dashboard.
2. Finance user selects tenant and billing period.
3. System shows included quota, total usage, overage, and estimated amount.
4. Finance user exports invoice-ready CSV or JSON.
5. External billing or ERP system consumes the export.

## 8. High-Level Architecture

```text
Consumer Apps / Partner Systems
        |
        v
API Gateway Edge
        |
        +-- Authentication and Authorization
        +-- Rate Limiting and Quotas
        +-- Tenant Resolver
        +-- Routing Engine
        +-- Transformation Engine
        +-- Billing Meter
        +-- Audit Logger
        |
        +--------------------+
        |                    |
        v                    v
REST / SOAP / gRPC / MQ    Protocol Connectors
Backend Services           ISO8583 / Files / TCP
                             |
                             v
                       Payment Switch /
                       Core Banking /
                       Card System
```

## 9. Suggested Services

### Gateway Runtime

Handles live API traffic.

Responsibilities:

- Resolve tenant.
- Authenticate request.
- Apply rate limit.
- Execute route.
- Run transformation.
- Forward request to backend.
- Record metrics and billing events.

### Transformation Service

Handles protocol and payload transformation.

Responsibilities:

- Parse ISO8583 messages.
- Build ISO8583 messages.
- Parse and build protocol-specific payloads through adapters.
- Map protocol-specific payloads to canonical transaction objects.
- Map canonical transaction objects to target protocol payloads.
- Transform JSON payloads.
- Validate mapping templates.
- Version transformation rules.

### Tenant Management Service

Handles tenant configuration.

Responsibilities:

- Tenant profile.
- Tenant users.
- Tenant credentials.
- Tenant quotas.
- Tenant API ownership.

### Billing Service

Handles usage metering and billing reports.

Responsibilities:

- Consume gateway usage events.
- Aggregate usage by billing period.
- Apply pricing plans.
- Generate invoice-ready records.
- Export billing data.

### Admin UI

Optional browser UI used by internal platform operators and tenant admins after the control plane APIs are stable.

Responsibilities:

- API configuration.
- Tenant configuration.
- Transformation templates.
- Billing reports.
- Audit logs.

### Developer-Facing UI

Optional browser UI used by API consumers after developer-facing API workflows are stable.

Responsibilities:

- API documentation.
- Credential management.
- Usage visibility.
- Sample requests.

## 10. Data Model

Core entities:

```text
Tenant
- id
- name
- status
- billingPlanId
- createdAt
- updatedAt

User
- id
- email
- name
- status

TenantUser
- id
- tenantId
- userId
- role
- status

ApiProduct
- id
- tenantId
- name
- description
- status

Route
- id
- tenantId
- apiProductId
- name
- inboundProtocol
- outboundProtocol
- host
- method
- path
- listenerRef
- upstreamId
- transformationTemplateId
- rateLimitPolicyId
- quotaPolicyId
- priority
- timeoutMs
- status

ProtocolAdapterConfig
- id
- tenantId
- name
- protocol
- direction
- config
- status

TransformationTemplate
- id
- tenantId
- apiProductId
- name
- direction
- version
- templateBody
- status

Credential
- id
- tenantId
- consumerId
- type
- keyPrefix
- secretHash
- secretRef
- status

UsageEvent
- id
- tenantId
- apiProductId
- consumerId
- routeId
- sourceProtocol
- targetProtocol
- statusCode
- transformationType
- latencyMs
- billable
- occurredAt

BillingPlan
- id
- name
- monthlyFee
- includedRequests
- overagePrice
- currency

BillingSummary
- id
- tenantId
- billingPeriod
- requestCount
- successCount
- failureCount
- includedQuota
- billableOverage
- estimatedAmount
- currency
```

## 11. Multitenancy Design

The gateway should use logical multitenancy in the MVP.

Tenant isolation requirements:

- Every API configuration belongs to exactly one tenant.
- Every credential belongs to exactly one tenant.
- Every route belongs to exactly one tenant.
- Every transformation template belongs to exactly one tenant.
- Every usage event includes tenant ID.
- Every billing summary is generated per tenant.
- Admin access must be scoped by tenant role.

Tenant resolution options:

- Subdomain: `tenant-a.gateway.example.com`
- Header: `X-Tenant-ID: tenant-a`
- API key ownership.
- OAuth2 client ownership.

Recommended MVP approach:

- Resolve tenant from API key or OAuth2 client.
- Optionally support tenant subdomain for cleaner partner-facing APIs.

## 12. Compliance and Risk Considerations

Finance companies require stronger controls than general API platforms.

Important considerations:

- PCI DSS scope awareness for card data.
- Strong encryption in transit.
- Encryption at rest for sensitive configuration.
- PAN masking.
- Key rotation.
- Audit logs for every admin action.
- Separation between operational logs and sensitive transaction payloads.
- Role-based access control.
- Least-privilege service access.
- Data retention policy per tenant.

## 13. MVP Success Metrics

Product success:

- A tenant can publish a REST API backed by ISO8583.
- A tenant can publish an ISO8583 listener backed by REST.
- A tenant can publish at least one additional protocol route through the adapter model in a controlled test environment.
- A consumer can call a secured API successfully.
- Gateway records usage and produces billing summaries.
- Admin users can trace request failures.
- Transformation templates can be updated without redeploying the gateway.

Operational success:

- P95 gateway latency overhead below 100 ms excluding upstream latency.
- 99.9% gateway availability for MVP environment.
- Transformation error rate below 0.1% after template validation.
- Billing event loss rate of 0%.

Business success:

- First tenant onboarded in less than 1 day.
- First transformed API published in less than 2 hours.
- Billing report generated automatically at period end.

## 14. MVP Release Phases

### Phase 1: Core Gateway

- REST routing.
- Protocol adapter interfaces.
- Tenant resolution.
- API key authentication.
- Rate limiting.
- Basic logs and metrics.

### Phase 2: Transformation

- REST to ISO8583 transformation.
- ISO8583 to REST transformation.
- REST to REST transformation or pass-through.
- Canonical transformation object.
- Template versioning.
- Transformation testing tool.

### Phase 3: Billing

- Usage event collection.
- Billing aggregation.
- Pricing plans.
- Billing reports and exports.

### Phase 4: Portal

- Control plane APIs for admin workflows.
- Developer-facing API documentation and credentials.
- API documentation.
- Tenant usage dashboard.
- Browser-based admin and developer portals after API workflows are stable.

## 15. Recommended MVP Acceptance Criteria

The MVP is complete when:

- A platform admin can create a tenant.
- A tenant admin can configure an API route.
- A tenant admin can configure an ISO8583 upstream connection.
- A tenant admin can create a REST-to-ISO8583 mapping.
- A tenant admin can create an ISO8583-to-REST mapping.
- A tenant admin can configure routes using the protocol adapter model.
- A consumer can authenticate and call the API.
- Gateway can transform a REST request into ISO8583 and return JSON response.
- Gateway can receive ISO8583, call REST backend, and return ISO8583 response.
- Gateway records tenant-scoped usage events.
- Billing service generates tenant billing summaries.
- Logs mask sensitive cardholder data.
- Audit logs capture configuration changes.

## 16. Future Enhancements

Potential post-MVP features:

- API marketplace.
- Partner onboarding workflow.
- Webhook support.
- Full SOAP support.
- gRPC support.
- GraphQL facade support.
- Message queue adapters.
- File-based adapters for CSV, fixed-width, and SFTP batch exchange.
- Proprietary TCP adapter SDK.
- Event streaming support.
- Advanced ISO8583 dialect management.
- HSM integration.
- mTLS per partner.
- OpenAPI import and export.
- SDK generation.
- Real-time billing dashboard.
- Automated invoice generation.
- Payment collection integration.
- SLA management.
- Multi-region deployment.
- Dedicated tenant deployment model.
- Fraud and risk scoring integration.
