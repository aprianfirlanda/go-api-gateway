# Billing Design: Multitenant API Gateway

## 1. Purpose

This document defines the billing and usage metering design for the multitenant API gateway.

The gateway must support usage-based billing for finance companies and their consumers. Billing must be tenant-aware, auditable, replayable, and independent from application logs.

## 2. Billing Goals

- Meter every request attempt.
- Attribute usage to tenant, consumer, API product, route, and protocol.
- Support free quota, fixed monthly fee, and overage pricing.
- Support billing summaries by billing period.
- Support invoice-ready exports.
- Make billing aggregation replayable from raw usage events.
- Avoid storing sensitive payload data in billing records.
- Support future pricing by protocol, route, success count, transaction type, and SLA tier.

## 3. Non-Goals for MVP

Out of scope for MVP:

- Payment collection.
- Tax calculation.
- Full accounting ledger.
- Revenue recognition.
- Subscription lifecycle automation.
- Dunning and failed payment workflows.
- Multi-currency settlement.

The MVP should produce invoice-ready usage data that can be consumed by an external billing, ERP, finance, or accounting system.

## 4. Billing Concepts

### 4.1 Tenant

The organization being billed.

Example:

```text
Bank A
Fintech B
Merchant Group C
```

### 4.2 Consumer

The application, partner, merchant, or internal system consuming APIs under a tenant.

Example:

```text
mobile-banking-app
partner-switch
merchant-aggregator
```

### 4.3 API Product

A commercial API grouping.

Example:

```text
Card Authorization
Account Inquiry
Fund Transfer
Settlement File Upload
```

### 4.4 Route

The technical route that handled traffic.

Example:

```text
POST /cards/authorization
ISO8583 listener card-auth-primary
```

### 4.5 Usage Event

An immutable event emitted by the gateway for a request attempt or billable processing unit.

### 4.6 Billing Summary

Aggregated usage and estimated charges for one tenant and billing period.

## 5. Billable Units

MVP billable unit:

```text
API request attempt
```

Future billable units:

- Successful transaction.
- ISO8583 transaction.
- SOAP/XML request.
- gRPC call.
- Webhook delivery.
- Queue message.
- File batch.
- File record.
- Data volume.
- Dedicated tenant runtime.
- Premium SLA tier.

## 6. Usage Event Schema

Every request attempt should produce one usage event.

Schema:

```json
{
  "eventId": "evt_01HX000001",
  "tenantId": "tenant_bank_a",
  "consumerId": "mobile_app",
  "apiProductId": "card_authorization",
  "routeId": "route_purchase_auth",
  "sourceProtocol": "rest",
  "targetProtocol": "iso8583",
  "transformationType": "rest_to_iso8583",
  "status": "success",
  "httpStatus": 200,
  "upstreamStatus": "00",
  "latencyMs": 87,
  "billable": true,
  "occurredAt": "2026-05-08T10:30:00Z"
}
```

Required fields:

- `eventId`
- `tenantId`
- `sourceProtocol`
- `targetProtocol`
- `status`
- `latencyMs`
- `billable`
- `occurredAt`

Recommended fields:

- `consumerId`
- `apiProductId`
- `routeId`
- `transformationType`
- `httpStatus`
- `upstreamStatus`

Usage events must not contain:

- Full request payload.
- Full response payload.
- PAN.
- CVV.
- PIN block.
- API keys.
- Access tokens.
- Private customer data.

## 7. Event Status

Supported event statuses:

```text
success
failed
rejected
timeout
```

Definitions:

- `success`: gateway completed request and returned a successful response according to route policy.
- `failed`: request failed after processing started.
- `rejected`: request was rejected by authentication, authorization, rate limit, quota, validation, or policy.
- `timeout`: upstream or protocol operation timed out.

## 8. Billable Flag Rules

The `billable` flag should be calculated by policy.

Default MVP rules:

```text
success: billable
failed: billable if upstream was called
rejected: not billable
timeout: billable if upstream was called
```

Examples:

```text
Invalid API key -> rejected -> not billable
Rate limit exceeded -> rejected -> not billable
Transformation validation error before upstream -> failed -> not billable
ISO8583 upstream timeout after sending message -> timeout -> billable
REST upstream returns 500 -> failed -> billable
```

Tenant-specific billing policies may override these rules later.

## 9. Pricing Plan Model

MVP pricing plan fields:

```text
id
name
slug
monthly_fee
included_requests
overage_price
currency
status
```

Example:

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

## 10. Pricing Models

### 10.1 MVP Pricing

MVP should support:

- Fixed monthly fee.
- Included request quota.
- Overage price per request.

Formula:

```text
billable_overage = max(billable_count - included_requests, 0)
overage_amount = billable_overage * overage_price
estimated_amount = monthly_fee + overage_amount
```

### 10.2 Future Pricing

Future pricing can support:

- Price by API product.
- Price by route.
- Price by protocol.
- Price by successful transaction only.
- Price by file record.
- Price by batch.
- Price by data volume.
- Price by dedicated runtime.
- Tiered overage pricing.
- Volume discounts.

## 11. Billing Period

MVP billing period:

```text
Monthly, based on UTC calendar month
```

Billing period format:

```text
YYYY-MM
```

Example:

```text
2026-05
```

Future:

- Tenant-specific billing timezone.
- Custom billing cycle start day.
- Proration.

## 12. Billing Aggregation

Billing worker aggregates usage events into billing summaries.

Aggregation dimensions:

- Tenant.
- Billing period.
- Billing plan.

MVP summary fields:

```text
request_count
success_count
failure_count
rejected_count
timeout_count
billable_count
included_quota
billable_overage
monthly_fee
overage_amount
estimated_amount
currency
```

Aggregation formula:

```text
request_count = count(*)
success_count = count(status = 'success')
failure_count = count(status = 'failed')
rejected_count = count(status = 'rejected')
timeout_count = count(status = 'timeout')
billable_count = count(billable = true)
included_quota = billing_plan.included_requests
billable_overage = max(billable_count - included_quota, 0)
monthly_fee = billing_plan.monthly_fee
overage_amount = billable_overage * billing_plan.overage_price
estimated_amount = monthly_fee + overage_amount
```

## 13. Billing Summary Example

```json
{
  "tenantId": "tenant_bank_a",
  "billingPeriod": "2026-05",
  "billingPlanId": "enterprise",
  "requestCount": 12500000,
  "successCount": 12300000,
  "failureCount": 120000,
  "rejectedCount": 50000,
  "timeoutCount": 30000,
  "billableCount": 12450000,
  "includedQuota": 10000000,
  "billableOverage": 2450000,
  "monthlyFee": 5000,
  "overageAmount": 735,
  "estimatedAmount": 5735,
  "currency": "USD",
  "status": "draft",
  "calculatedAt": "2026-06-01T00:10:00Z"
}
```

## 14. Usage Breakdown

Billing reports should support breakdowns by:

- API product.
- Consumer.
- Route.
- Source protocol.
- Target protocol.
- Status.
- Day.

Example API product breakdown:

```json
{
  "apiProductId": "card_authorization",
  "requestCount": 9000000,
  "billableCount": 8950000,
  "successCount": 8800000,
  "estimatedShare": 4120.5
}
```

## 15. Event Delivery Design

The gateway must not lose usage events in normal operation.

Recommended MVP:

- Gateway writes usage event to PostgreSQL.
- If direct write fails, write to local durable outbox where available.
- Billing worker aggregates from PostgreSQL.

Recommended production:

- Gateway writes usage event to durable queue or outbox.
- Worker persists usage events.
- Billing worker aggregates persisted events.

Possible event flow:

```text
Gateway Runtime
  -> Usage Event
  -> Outbox / Durable Queue
  -> Usage Event Writer
  -> usage_events table
  -> Billing Aggregator
  -> billing_summaries table
```

## 16. Replay Strategy

Billing aggregation must be replayable.

Rules:

- Raw usage events are immutable.
- Draft billing summaries can be recalculated.
- Finalized billing summaries require explicit admin action to recalculate.
- Recalculation must create an audit log.
- Recalculation should store previous and new totals.

Replay steps:

1. Select tenant and billing period.
2. Load billing plan effective for period.
3. Aggregate raw usage events.
4. Replace draft summary or create adjustment for finalized summary.
5. Write audit event.

## 17. Finalization

Billing summary statuses:

```text
draft
finalized
exported
```

Status rules:

- `draft`: can be recalculated.
- `finalized`: locked for invoice generation.
- `exported`: exported to finance system.

Finalization should be explicit.

```http
POST /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}/finalize
```

## 18. Export Design

MVP export formats:

- CSV.
- JSON.

CSV export should include:

```text
tenant_id
billing_period
billing_plan
request_count
success_count
failure_count
rejected_count
timeout_count
billable_count
included_quota
billable_overage
monthly_fee
overage_amount
estimated_amount
currency
status
calculated_at
```

JSON export should include:

- Summary.
- API product breakdown.
- Consumer breakdown.
- Protocol breakdown.
- Generated timestamp.

## 19. Protocol-Aware Billing

MVP records protocol dimensions even if pricing does not vary by protocol yet.

Fields:

```text
source_protocol
target_protocol
transformation_type
```

This enables future pricing such as:

```text
REST to REST: 0.0001 USD/request
REST to ISO8583: 0.0003 USD/request
SOAP/XML to REST: 0.0002 USD/request
File record processing: 0.00005 USD/record
```

## 20. Quota vs Billing

Quotas and billing are related but separate.

Quota:

- Runtime enforcement.
- Protects contract limits.
- Can reject requests.
- Uses counters during request processing.

Billing:

- Financial reporting.
- Uses immutable usage events.
- Aggregates after request processing.
- Produces invoice-ready data.

Example:

```text
Monthly quota: 10,000,000 requests
Exceeded behavior: allow_overage
Billing plan: 10,000,000 included, 0.0003 USD overage
```

In this case, quota allows traffic after the threshold and billing charges overage.

## 21. Data Retention

Recommended retention:

```text
usage_events: 13 months
billing_summaries: 7 years
billing_exports: 7 years
audit_logs for billing changes: 7 years
```

Retention should be configurable for enterprise tenants.

## 22. Security and Compliance

Billing records must not contain sensitive payload data.

Do not store:

- PAN.
- CVV.
- PIN block.
- Full account number.
- API key.
- Access token.

Billing admin actions must be audited:

- Plan changed.
- Summary recalculated.
- Summary finalized.
- Summary exported.
- Manual adjustment created.

## 23. Failure Handling

Usage event write failure:

- Do not fail the customer request only because billing write failed.
- Record event in durable retry path.
- Emit alert if retry backlog grows.

Billing aggregation failure:

- Mark job failed.
- Keep previous summary unchanged.
- Retry safely.
- Alert operations team.

Export failure:

- Keep billing summary status unchanged.
- Return error to caller.
- Write audit event for failed export attempt if needed.

## 24. Billing Worker Jobs

Recommended jobs:

```text
usage_event_retry_worker
hourly_usage_aggregation_worker
monthly_billing_summary_worker
billing_recalculation_worker
billing_export_worker
```

MVP can start with:

- `monthly_billing_summary_worker`
- `billing_recalculation_worker`

## 25. Billing APIs

Required APIs:

```text
POST /admin/v1/billing-plans
GET  /admin/v1/billing-plans
GET  /admin/v1/tenants/{tenantId}/usage
GET  /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}
POST /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}/recalculate
POST /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}/finalize
GET  /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}/export
```

## 26. Testing Strategy

Unit tests:

- Billable flag calculation.
- Billing period calculation.
- Overage calculation.
- Pricing formula.
- Usage event validation.
- Export formatting.

Integration tests:

- Gateway emits usage event.
- Billing worker aggregates usage events.
- Draft summary recalculates.
- Finalized summary requires explicit recalculation.
- Tenant A usage does not affect Tenant B.
- Protocol breakdown is correct.

Load tests:

- Usage event write throughput.
- Billing aggregation over large event volume.

## 27. Open Decisions

Open billing decisions:

- Whether usage events go directly to PostgreSQL or through a queue from day one.
- Whether failed requests should be billable by default for all tenants.
- Whether billing plans should support tenant-specific negotiated prices in MVP.
- Whether billing summaries should include daily rollups.
- Whether invoice exports should include tax fields.
- Whether file batches are billed by batch, record, or both.

