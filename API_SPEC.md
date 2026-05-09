# API Spec: Multitenant API Gateway Control Plane

## 1. Purpose

This document defines the initial API specification for the gateway control plane and runtime-facing APIs.

The control plane manages:

- Tenants.
- Users and roles.
- API products.
- Routes.
- Upstreams.
- Protocol adapter configs.
- ISO8583 profiles.
- Transformation templates.
- Consumers and credentials.
- Rate limits and quotas.
- Billing plans and usage reports.
- Audit logs.

The data plane handles live API traffic and should use compiled runtime configuration from the control plane.

## 2. API Conventions

Base path:

```text
/admin/v1
```

Content type:

```http
Content-Type: application/json
```

Timestamps:

```text
RFC3339 / ISO-8601 UTC
```

IDs:

```text
UUID
```

Pagination query parameters:

```text
limit
cursor
```

Recommended default:

```text
limit=50
max limit=200
```

Standard list response:

```json
{
  "data": [],
  "nextCursor": null
}
```

## 3. Authentication

Control plane APIs require admin authentication.

MVP options:

- Platform admin token for internal setup.
- Tenant admin API session or JWT.

Required request header:

```http
Authorization: Bearer <token>
```

Authorization must enforce:

- Platform admins can access all tenants.
- Tenant admins can access only their tenant.
- Billing viewers can read billing data only.
- Auditors can read audit logs only.
- Developers can manage credentials and view API docs only, based on tenant policy.

## 4. Error Format

All APIs should return a consistent error body.

```json
{
  "error": {
    "code": "validation_error",
    "message": "Invalid request body",
    "details": [
      {
        "field": "name",
        "message": "name is required"
      }
    ],
    "requestId": "req_01HX000001"
  }
}
```

Common error codes:

```text
validation_error
unauthorized
forbidden
not_found
conflict
rate_limited
quota_exceeded
upstream_timeout
transformation_error
internal_error
```

## 5. Tenant APIs

### 5.1 Create Tenant

```http
POST /admin/v1/tenants
```

Request:

```json
{
  "name": "Bank A",
  "slug": "bank-a",
  "billingPlanId": "7a35c4ad-d0e9-4b6c-9f17-447efc1df98f",
  "metadata": {
    "industry": "banking"
  }
}
```

Response:

```json
{
  "id": "tenant_uuid",
  "name": "Bank A",
  "slug": "bank-a",
  "status": "active",
  "billingPlanId": "7a35c4ad-d0e9-4b6c-9f17-447efc1df98f",
  "createdAt": "2026-05-08T00:00:00Z",
  "updatedAt": "2026-05-08T00:00:00Z"
}
```

### 5.2 List Tenants

```http
GET /admin/v1/tenants?limit=50&cursor=
```

### 5.3 Get Tenant

```http
GET /admin/v1/tenants/{tenantId}
```

### 5.4 Update Tenant

```http
PATCH /admin/v1/tenants/{tenantId}
```

Request:

```json
{
  "name": "Bank A Indonesia",
  "status": "active",
  "billingPlanId": "plan_uuid"
}
```

## 6. User and Role APIs

### 6.1 Add User to Tenant

```http
POST /admin/v1/tenants/{tenantId}/users
```

Request:

```json
{
  "email": "admin@banka.example",
  "name": "Bank A Admin",
  "role": "tenant_admin"
}
```

### 6.2 List Tenant Users

```http
GET /admin/v1/tenants/{tenantId}/users
```

### 6.3 Update Tenant User Role

```http
PATCH /admin/v1/tenants/{tenantId}/users/{userId}
```

Request:

```json
{
  "role": "api_operator",
  "status": "active"
}
```

### 6.4 Remove User from Tenant

```http
DELETE /admin/v1/tenants/{tenantId}/users/{userId}
```

## 7. API Product APIs

### 7.1 Create API Product

```http
POST /admin/v1/tenants/{tenantId}/api-products
```

Request:

```json
{
  "name": "Card Authorization",
  "slug": "card-authorization",
  "description": "Card purchase authorization API"
}
```

### 7.2 List API Products

```http
GET /admin/v1/tenants/{tenantId}/api-products
```

### 7.3 Get API Product

```http
GET /admin/v1/tenants/{tenantId}/api-products/{apiProductId}
```

### 7.4 Update API Product

```http
PATCH /admin/v1/tenants/{tenantId}/api-products/{apiProductId}
```

Request:

```json
{
  "name": "Card Authorization",
  "description": "Updated description",
  "status": "active"
}
```

## 8. Upstream APIs

### 8.1 Create Upstream

```http
POST /admin/v1/tenants/{tenantId}/upstreams
```

REST upstream request:

```json
{
  "name": "core-banking-rest",
  "protocol": "rest",
  "config": {
    "baseUrl": "https://core.example.com",
    "tls": {
      "verify": true
    }
  },
  "secretRef": null
}
```

ISO8583 upstream request:

```json
{
  "name": "switch-primary",
  "protocol": "iso8583",
  "config": {
    "host": "10.10.10.20",
    "port": 5000,
    "connectionMode": "persistent",
    "profileId": "iso_profile_uuid"
  }
}
```

### 8.2 List Upstreams

```http
GET /admin/v1/tenants/{tenantId}/upstreams
```

### 8.3 Update Upstream

```http
PATCH /admin/v1/tenants/{tenantId}/upstreams/{upstreamId}
```

## 9. Route APIs

### 9.1 Create Route

```http
POST /admin/v1/tenants/{tenantId}/routes
```

REST to ISO8583 request:

```json
{
  "apiProductId": "api_product_uuid",
  "name": "Purchase Authorization",
  "inboundProtocol": "rest",
  "outboundProtocol": "iso8583",
  "host": "api.gateway.example.com",
  "method": "POST",
  "path": "/cards/authorization",
  "upstreamId": "switch_upstream_uuid",
  "transformationTemplateId": "template_uuid",
  "rateLimitPolicyId": "rate_policy_uuid",
  "quotaPolicyId": "quota_policy_uuid",
  "priority": 100,
  "timeoutMs": 5000,
  "status": "draft"
}
```

SOAP/XML to REST request:

```json
{
  "apiProductId": "api_product_uuid",
  "name": "Account Inquiry SOAP Compatibility",
  "inboundProtocol": "soap_xml",
  "outboundProtocol": "rest",
  "host": "api.gateway.example.com",
  "method": "POST",
  "path": "/soap/account-inquiry",
  "upstreamId": "rest_upstream_uuid",
  "transformationTemplateId": "template_uuid",
  "priority": 100,
  "timeoutMs": 5000,
  "status": "draft"
}
```

### 9.2 List Routes

```http
GET /admin/v1/tenants/{tenantId}/routes
```

Optional filters:

```text
apiProductId
inboundProtocol
outboundProtocol
status
```

### 9.3 Get Route

```http
GET /admin/v1/tenants/{tenantId}/routes/{routeId}
```

### 9.4 Update Route

```http
PATCH /admin/v1/tenants/{tenantId}/routes/{routeId}
```

### 9.5 Publish Route

```http
POST /admin/v1/tenants/{tenantId}/routes/{routeId}/publish
```

### 9.6 Disable Route

```http
POST /admin/v1/tenants/{tenantId}/routes/{routeId}/disable
```

## 10. Protocol Adapter Config APIs

### 10.1 Create Protocol Adapter Config

```http
POST /admin/v1/tenants/{tenantId}/protocol-adapters
```

SOAP/XML example:

```json
{
  "name": "soap-xml-default",
  "protocol": "soap_xml",
  "direction": "both",
  "config": {
    "soapVersion": "1.1",
    "namespaces": {
      "soapenv": "http://schemas.xmlsoap.org/soap/envelope/",
      "bank": "http://example.com/bank"
    }
  },
  "status": "active"
}
```

File adapter example:

```json
{
  "name": "daily-settlement-csv",
  "protocol": "file",
  "direction": "inbound",
  "config": {
    "format": "csv",
    "delimiter": ",",
    "hasHeader": true,
    "source": "sftp"
  },
  "status": "active"
}
```

### 10.2 List Protocol Adapter Configs

```http
GET /admin/v1/tenants/{tenantId}/protocol-adapters
```

### 10.3 Update Protocol Adapter Config

```http
PATCH /admin/v1/tenants/{tenantId}/protocol-adapters/{adapterConfigId}
```

## 11. ISO8583 Profile APIs

### 11.1 Create ISO8583 Profile

```http
POST /admin/v1/tenants/{tenantId}/iso8583-profiles
```

Request:

```json
{
  "name": "default-switch-profile",
  "encoding": "ascii",
  "lengthHeaderEnabled": true,
  "lengthHeaderSizeBytes": 2,
  "lengthHeaderByteOrder": "big_endian",
  "bitmapEncoding": "binary",
  "fields": {
    "2": {
      "name": "pan",
      "type": "numeric",
      "lengthType": "llvar",
      "maxLength": 19,
      "sensitive": true
    },
    "3": {
      "name": "processing_code",
      "type": "numeric",
      "lengthType": "fixed",
      "length": 6
    },
    "4": {
      "name": "amount_transaction",
      "type": "numeric",
      "lengthType": "fixed",
      "length": 12
    }
  },
  "status": "draft"
}
```

### 11.2 List ISO8583 Profiles

```http
GET /admin/v1/tenants/{tenantId}/iso8583-profiles
```

### 11.3 Publish ISO8583 Profile

```http
POST /admin/v1/tenants/{tenantId}/iso8583-profiles/{profileId}/publish
```

## 12. Transformation Template APIs

### 12.1 Create Transformation Template

```http
POST /admin/v1/tenants/{tenantId}/transformation-templates
```

Request:

```json
{
  "apiProductId": "api_product_uuid",
  "name": "card-authorization-rest-to-iso8583",
  "sourceProtocol": "rest",
  "targetProtocol": "iso8583",
  "templateBody": {
    "request": {
      "fields": {
        "2": "$.fields.pan",
        "3": "'000000'",
        "4": "formatAmount($.fields.amount)",
        "41": "$.fields.terminalId",
        "49": "currencyNumeric($.fields.currency)"
      }
    },
    "response": {
      "fields": {
        "responseCode": "$.fields.39",
        "authorizationCode": "$.fields.38",
        "stan": "$.fields.11",
        "rrn": "$.fields.37"
      }
    }
  }
}
```

### 12.2 List Transformation Templates

```http
GET /admin/v1/tenants/{tenantId}/transformation-templates
```

Optional filters:

```text
sourceProtocol
targetProtocol
apiProductId
status
```

### 12.3 Test Transformation Template

```http
POST /admin/v1/tenants/{tenantId}/transformation-templates/{templateId}/test
```

Request:

```json
{
  "direction": "request",
  "input": {
    "fields": {
      "transactionType": "purchase",
      "pan": "4111111111111111",
      "amount": 10000,
      "currency": "IDR",
      "terminalId": "ATM00101"
    }
  }
}
```

Response:

```json
{
  "output": {
    "2": "4111111111111111",
    "3": "000000",
    "4": "000000010000",
    "41": "ATM00101",
    "49": "360"
  },
  "warnings": []
}
```

### 12.4 Publish Transformation Template

```http
POST /admin/v1/tenants/{tenantId}/transformation-templates/{templateId}/publish
```

## 13. Consumer and Credential APIs

### 13.1 Create Consumer

```http
POST /admin/v1/tenants/{tenantId}/consumers
```

Request:

```json
{
  "name": "Mobile Banking App",
  "slug": "mobile-banking-app",
  "ownerUserId": "user_uuid"
}
```

### 13.2 Grant API Product Access

```http
POST /admin/v1/tenants/{tenantId}/consumers/{consumerId}/api-access
```

Request:

```json
{
  "apiProductId": "api_product_uuid"
}
```

### 13.3 Create API Key Credential

```http
POST /admin/v1/tenants/{tenantId}/consumers/{consumerId}/credentials
```

Request:

```json
{
  "type": "api_key",
  "scopes": [
    "api:card-authorization:invoke"
  ],
  "expiresAt": "2027-05-08T00:00:00Z"
}
```

Response:

```json
{
  "id": "credential_uuid",
  "type": "api_key",
  "keyPrefix": "gw_live_abc123",
  "apiKey": "gw_live_abc123.full_secret_value_returned_once",
  "status": "active",
  "expiresAt": "2027-05-08T00:00:00Z"
}
```

The full API key must only be returned once.

### 13.4 Rotate Credential

```http
POST /admin/v1/tenants/{tenantId}/credentials/{credentialId}/rotate
```

### 13.5 Revoke Credential

```http
POST /admin/v1/tenants/{tenantId}/credentials/{credentialId}/revoke
```

## 14. Rate Limit and Quota APIs

### 14.1 Create Rate Limit Policy

```http
POST /admin/v1/tenants/{tenantId}/rate-limit-policies
```

Request:

```json
{
  "name": "default-card-auth-limit",
  "scope": "route",
  "limitCount": 500,
  "windowSeconds": 1,
  "burstCount": 100,
  "status": "active"
}
```

### 14.2 Create Quota Policy

```http
POST /admin/v1/tenants/{tenantId}/quota-policies
```

Request:

```json
{
  "name": "monthly-card-auth-quota",
  "scope": "api_product",
  "period": "monthly",
  "quotaCount": 50000000,
  "exceededBehavior": "allow_overage",
  "status": "active"
}
```

## 15. Billing APIs

### 15.1 Create Billing Plan

```http
POST /admin/v1/billing-plans
```

Request:

```json
{
  "name": "Enterprise",
  "slug": "enterprise",
  "monthlyFee": 5000,
  "includedRequests": 10000000,
  "overagePrice": 0.0003,
  "currency": "USD",
  "status": "active"
}
```

### 15.2 List Billing Plans

```http
GET /admin/v1/billing-plans
```

### 15.3 Get Tenant Usage

```http
GET /admin/v1/tenants/{tenantId}/usage?from=2026-05-01T00:00:00Z&to=2026-06-01T00:00:00Z
```

Optional filters:

```text
apiProductId
consumerId
routeId
sourceProtocol
targetProtocol
billable
```

### 15.4 Get Billing Summary

```http
GET /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}
```

Example:

```http
GET /admin/v1/tenants/{tenantId}/billing-summaries/2026-05
```

### 15.5 Recalculate Billing Summary

```http
POST /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}/recalculate
```

### 15.6 Finalize Billing Summary

```http
POST /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}/finalize
```

### 15.7 Export Billing Summary

```http
GET /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}/export?format=csv
```

Supported formats:

- `csv`
- `json`

## 16. Audit Log APIs

### 16.1 List Audit Logs

```http
GET /admin/v1/tenants/{tenantId}/audit-logs
```

Optional filters:

```text
actorUserId
action
resourceType
resourceId
from
to
```

Response:

```json
{
  "data": [
    {
      "id": "audit_uuid",
      "tenantId": "tenant_uuid",
      "actorUserId": "user_uuid",
      "actorRole": "tenant_admin",
      "action": "route.created",
      "resourceType": "route",
      "resourceId": "route_uuid",
      "occurredAt": "2026-05-08T00:00:00Z"
    }
  ],
  "nextCursor": null
}
```

## 17. Config Version APIs

### 17.1 Publish Tenant Config

```http
POST /admin/v1/tenants/{tenantId}/config/publish
```

Response:

```json
{
  "tenantId": "tenant_uuid",
  "version": 42,
  "checksum": "sha256:example",
  "publishedAt": "2026-05-08T00:00:00Z"
}
```

### 17.2 Get Runtime Config Snapshot

Used by gateway data plane instances.

```http
GET /admin/v1/runtime/config?tenantId={tenantId}&version=latest
```

Response:

```json
{
  "tenantId": "tenant_uuid",
  "version": 42,
  "routes": [],
  "upstreams": [],
  "protocolAdapters": [],
  "transformationTemplates": [],
  "iso8583Profiles": [],
  "rateLimitPolicies": [],
  "quotaPolicies": []
}
```

## 18. Data Plane Runtime APIs

These APIs are served by the gateway runtime.

### 18.1 Health Check

```http
GET /healthz
```

Response:

```json
{
  "status": "ok"
}
```

### 18.2 Readiness Check

```http
GET /readyz
```

Response:

```json
{
  "status": "ready",
  "configVersion": 42
}
```

### 18.3 Metrics

```http
GET /metrics
```

Prometheus-compatible metrics.

## 19. Public Gateway Request Behavior

Example REST request through gateway:

```http
POST /cards/authorization HTTP/1.1
Host: api.gateway.example.com
Authorization: Bearer gw_live_example
Content-Type: application/json
X-Request-ID: req_client_optional
```

Gateway behavior:

1. Assign or accept request ID.
2. Resolve tenant from credential.
3. Authenticate credential.
4. Authorize consumer for API product and route.
5. Apply IP allowlist.
6. Apply rate limit.
7. Apply quota policy.
8. Decode inbound protocol.
9. Transform request.
10. Call upstream.
11. Transform response.
12. Encode outbound protocol response.
13. Emit metrics.
14. Emit usage event.

## 20. Idempotency

For financial APIs, selected routes should support idempotency keys.

Request header:

```http
Idempotency-Key: unique-client-key
```

Recommended behavior:

- Store idempotency key by tenant, consumer, route, and key.
- Return the same response for retried identical requests.
- Reject key reuse with a different request hash.

This can be post-MVP unless a first customer requires it.

## 21. Open Decisions

Open API decisions:

- Whether control plane should expose OpenAPI documentation from day one.
- Whether public developer portal APIs should use a separate `/developer/v1` base path.
- Whether runtime config snapshots should be pulled by data plane or pushed through a queue.
- Whether credential creation should support approval workflow.
- Whether billing export should support invoice line-item grouping by protocol.
