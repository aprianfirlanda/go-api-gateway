# Security Design: Multitenant Finance API Gateway

## 1. Purpose

This document defines the security model for the Go-based multitenant API gateway.

The gateway is designed for finance companies, so it must assume sensitive traffic, regulated data, partner integrations, and strict audit requirements.

Security goals:

- Protect tenant data and configuration.
- Prevent cross-tenant access.
- Secure public and partner-facing APIs.
- Protect credentials and secrets.
- Mask sensitive financial data.
- Produce reliable audit logs.
- Support finance compliance requirements such as PCI DSS scope awareness.

## 2. Security Principles

- Deny by default.
- Authenticate before route execution.
- Resolve tenant before loading tenant-owned configuration.
- Authorize every admin and runtime action.
- Never store full API keys.
- Never log full PAN, CVV, or PIN block.
- Keep billing data separate from sensitive payload data.
- Use least privilege for users, services, and database access.
- Prefer explicit configuration over implicit behavior.
- Make every sensitive configuration change auditable.

## 3. Threat Model

Primary threats:

- Credential theft.
- Tenant data leakage.
- Cross-tenant route access.
- Malicious partner traffic.
- Replay attacks.
- Request tampering.
- Sensitive data leakage in logs.
- Billing fraud through unmetered traffic.
- Configuration mistakes.
- Insider misuse through admin APIs.
- Protocol parsing bugs, especially for ISO8583, XML, and custom TCP.

High-risk areas:

- Authentication middleware.
- Tenant resolution.
- Route matching.
- Transformation templates.
- ISO8583 parsing and packing.
- SOAP/XML parsing.
- Credential storage.
- Admin APIs.
- Logs and audit trails.

## 4. Tenant Isolation

Tenant isolation is mandatory.

Rules:

- Every tenant-owned resource must include `tenant_id`.
- Every tenant-scoped query must filter by `tenant_id`.
- A credential can belong to only one tenant.
- A route can belong to only one tenant.
- A transformation template can belong to only one tenant.
- A protocol adapter config can belong to only one tenant.
- A usage event must include tenant ID.
- Billing summaries must be generated per tenant.

Runtime enforcement:

1. Authenticate credential.
2. Resolve tenant from credential, listener, or adapter binding.
3. Load only routes for that tenant.
4. Match route within that tenant.
5. Authorize consumer access to API product.

Public requests must not trust `X-Tenant-ID` unless the authenticated credential also belongs to that tenant.

## 5. Authentication

### 5.1 Runtime API Authentication

MVP authentication methods:

- API key.
- OAuth2 client credentials with JWT validation.

Post-MVP authentication methods:

- mTLS.
- HMAC request signing.
- Partner certificate authentication.

### 5.2 API Key Rules

API keys should have:

- Prefix for lookup.
- Random secret value.
- Hash stored in database.
- Expiration date.
- Status.
- Scopes.

Example format:

```text
gw_live_abcd1234.secret_random_value
```

Storage:

```text
key_prefix = gw_live_abcd1234
secret_hash = argon2id(secret_random_value)
```

Rules:

- Full API key is returned only once.
- Full API key is never stored.
- Hash must use Argon2id or bcrypt.
- Key prefix alone must not authenticate requests.
- API keys must support rotation and revocation.

### 5.3 OAuth2 and JWT Rules

JWT validation must check:

- Signature.
- Issuer.
- Audience.
- Expiration.
- Not-before.
- Client ID.
- Tenant binding.
- Scopes.

JWT claims should include:

```json
{
  "iss": "gateway-auth",
  "aud": "gateway-runtime",
  "sub": "client_uuid",
  "tenant_id": "tenant_uuid",
  "scope": "api:card-authorization:invoke",
  "exp": 1778198400
}
```

## 6. Authorization

### 6.1 Runtime Authorization

Runtime authorization checks:

- Credential is active.
- Tenant is active.
- Consumer is active.
- Consumer has access to API product.
- Route is active.
- Credential scopes allow route invocation.
- IP allowlist permits source IP.

### 6.2 Admin Authorization

Admin roles:

- `platform_admin`
- `tenant_admin`
- `api_operator`
- `billing_viewer`
- `developer`
- `auditor`

Role permissions:

```text
platform_admin: all tenants and platform settings
tenant_admin: manage tenant resources
api_operator: manage APIs, routes, upstreams, templates
billing_viewer: read billing and usage
developer: read API docs and manage own app credentials
auditor: read audit logs and configuration history
```

Every admin API must check:

- Authenticated user.
- Tenant membership.
- Role permission.
- Resource tenant ownership.

## 7. Transport Security

Required:

- TLS for all public HTTP APIs.
- Strong TLS versions and ciphers.
- TLS certificate rotation process.
- Internal service TLS where possible.

Post-MVP:

- mTLS for high-risk partners.
- mTLS for gateway-to-upstream connections.
- Certificate pinning for selected partner links.

Minimum target:

```text
TLS 1.2+
Prefer TLS 1.3
Disable weak ciphers
```

## 8. Request Signing

HMAC request signing should be supported for finance partners.

Signed components:

- HTTP method.
- Path.
- Query string.
- Request body hash.
- Timestamp.
- Nonce.

Example headers:

```http
X-Signature: hmac-sha256=...
X-Timestamp: 2026-05-08T10:00:00Z
X-Nonce: nonce_01HX000001
```

Validation rules:

- Reject stale timestamps.
- Reject reused nonce.
- Validate body hash.
- Validate signature with tenant credential.

Recommended timestamp tolerance:

```text
5 minutes
```

## 9. Replay Protection and Idempotency

Replay protection should be used with request signing.

Store nonce by:

```text
tenant_id
consumer_id
credential_id
nonce
```

Idempotency should be supported for selected financial routes.

Idempotency key scope:

```text
tenant_id
consumer_id
route_id
idempotency_key
```

Rules:

- Same key and same request hash returns same response.
- Same key and different request hash returns conflict.
- Store idempotency records with expiration.

## 10. Secret Management

Secret values should not be stored directly in normal database columns.

Use `secret_ref` for:

- OAuth client secrets.
- HMAC secrets.
- mTLS private keys.
- Upstream passwords.
- SOAP credentials.
- Queue credentials.
- SFTP credentials.

MVP option:

- Environment-backed or encrypted local secret storage.

Production option:

- Cloud KMS.
- Vault.
- Dedicated secret manager.
- HSM for sensitive payment cryptography.

## 11. Sensitive Data Handling

Sensitive data:

- PAN.
- CVV.
- PIN block.
- Account number.
- Customer identifiers.
- National ID.
- Access tokens.
- API keys.
- Client secrets.

Rules:

- Never log full PAN.
- Never log CVV.
- Never log PIN block.
- Never include sensitive payload values in billing events.
- Never include full credentials in audit logs.
- Mask sensitive values in transformation dry-run output by default.
- Use explicit allowlist if a support user needs payload visibility.

Masking examples:

```text
PAN: 4111111111111111 -> 411111******1111
Account: 1234567890 -> ******7890
API key: gw_live_abcd1234.secret -> gw_live_abcd1234.****
```

## 12. Logging Security

Application logs may include:

- Request ID.
- Tenant ID.
- Consumer ID.
- API product ID.
- Route ID.
- Source protocol.
- Target protocol.
- HTTP status.
- Upstream status.
- Latency.
- Error code.

Application logs must not include:

- Full request body by default.
- Full response body by default.
- Full PAN.
- CVV.
- PIN block.
- Full API key.
- Client secret.
- Private key.

Debug payload logging should be disabled in production unless there is a controlled, audited, temporary support workflow.

## 13. Audit Logging

Audit logs are mandatory for admin and configuration changes.

Audit events:

- Tenant created or updated.
- User added, role changed, or removed.
- Credential created, rotated, suspended, or revoked.
- API product created or changed.
- Route created, published, disabled, or deleted.
- Upstream changed.
- Protocol adapter config changed.
- ISO8583 profile changed.
- Transformation template published or rolled back.
- Billing plan changed.
- Rate limit or quota changed.
- Runtime config published.

Audit log must include:

```text
tenant_id
actor_user_id
actor_role
action
resource_type
resource_id
before
after
ip_address
user_agent
occurred_at
```

Sensitive values in `before` and `after` must be masked.

## 14. Admin API Security

Admin APIs must enforce:

- Authentication.
- RBAC.
- Tenant ownership checks.
- Request validation.
- Rate limits.
- Audit logging.
- Secure error messages.

Admin API responses must not expose:

- Secret hashes.
- Full API keys.
- Private keys.
- Raw secret refs if they reveal infrastructure details.

Credential creation is the only time a full generated API key can be returned.

## 15. Protocol Security

### 15.1 ISO8583

Risks:

- Invalid bitmap.
- Malformed variable length fields.
- Unexpected binary data.
- Sensitive field logging.
- Timeout ambiguity.

Controls:

- Validate message against profile.
- Enforce max message size.
- Enforce field length.
- Mask sensitive fields.
- Configure timeout response mapping.
- Log parse errors without raw sensitive payloads.

### 15.2 SOAP/XML

Risks:

- XML entity attacks.
- Oversized XML payloads.
- Namespace confusion.
- SOAP fault leakage.

Controls:

- Disable external entity resolution.
- Enforce XML size limits.
- Enforce parse timeouts.
- Validate namespaces.
- Sanitize SOAP faults before returning to consumers.

### 15.3 gRPC

Risks:

- Metadata leakage.
- Deadline abuse.
- Large message payloads.

Controls:

- Enforce message size limits.
- Enforce deadlines.
- Filter metadata forwarding.
- Map gRPC errors safely.

### 15.4 File and Batch

Risks:

- Malicious files.
- Oversized files.
- Formula injection in CSV exports.
- Partial failure confusion.

Controls:

- Enforce file size limits.
- Validate file type.
- Validate record count.
- Escape CSV export cells when needed.
- Track batch and record IDs.

## 16. Rate Limiting and Abuse Protection

Required:

- Tenant-level rate limit.
- Consumer-level rate limit.
- Route-level rate limit.
- Admin API rate limit.
- Authentication failure rate limit.

Abuse cases:

- Credential brute force.
- High-volume invalid requests.
- Transformation validation abuse.
- Large payload attacks.
- Repeated ISO8583 malformed messages.

Controls:

- Reject oversized payloads early.
- Rate limit failed authentication attempts.
- Apply per-source IP limits for unauthenticated traffic.
- Apply backpressure when upstreams are unhealthy.

## 17. Billing Security

Billing usage events are security-sensitive because they affect revenue.

Rules:

- Emit usage event for every request attempt.
- Usage events must include tenant ID.
- Usage events should be append-only.
- Billing aggregation should be replayable.
- Failed billing event writes must use durable retry.
- Admin recalculation of finalized billing summaries must be audited.

Usage events must not include sensitive payload values.

## 18. Database Security

Required:

- Least-privilege database users.
- Separate app user and migration user.
- TLS to database where supported.
- Backups encrypted at rest.
- Tenant-scoped indexes.
- No plain-text secrets.

Recommended production hardening:

- PostgreSQL Row-Level Security.
- Separate database schema for dedicated tenants.
- Separate database cluster for regulated enterprise tenants.
- Database activity monitoring.

## 19. CI/CD Security

Required:

- Unit tests.
- Integration tests.
- Static analysis.
- Dependency vulnerability scanning.
- Secret scanning.
- Container image scanning if using containers.

Deployment rules:

- No secrets in repository.
- No secrets in build logs.
- Environment-specific config.
- Rollback process.
- Migration review for destructive changes.

## 20. PCI DSS Scope Awareness

The gateway may process cardholder data if it handles PAN or payment authorization messages.

PCI-aware rules:

- Minimize cardholder data.
- Mask PAN wherever possible.
- Do not store CVV.
- Do not log PIN block.
- Restrict access to cardholder data.
- Encrypt sensitive data in transit.
- Encrypt sensitive data at rest where stored.
- Maintain audit logs.
- Rotate keys and credentials.
- Segment systems handling card data.

This document does not certify PCI compliance. It defines controls that reduce risk and support a future PCI assessment.

## 21. Incident Response Basics

Security events to alert on:

- Spike in authentication failures.
- Cross-tenant access attempt.
- Admin role changes.
- Credential rotation or revocation.
- Suspicious route changes.
- Billing event write failures.
- Repeated malformed ISO8583 messages.
- Repeated XML parse failures.
- Upstream timeout spike.

Minimum incident response data:

- Request ID.
- Tenant ID.
- Consumer ID.
- Credential ID or key prefix.
- Actor user ID for admin actions.
- Source IP.
- Route ID.
- Timestamp.
- Error code.

## 22. Security Acceptance Criteria

MVP security is acceptable when:

- API keys are hashed.
- Tenant isolation is enforced in runtime and admin APIs.
- Unauthorized requests are rejected.
- Cross-tenant route access is tested.
- Sensitive data masking is tested.
- Audit logs are written for admin changes.
- Credentials can be rotated and revoked.
- Rate limits work.
- Billing usage events cannot be skipped in normal request flow.
- ISO8583 malformed messages are rejected safely.
- SOAP/XML parser disables unsafe XML features before SOAP support is enabled.

## 23. Open Decisions

Open security decisions:

- Which secret manager to use in production.
- Whether to enable PostgreSQL Row-Level Security from MVP.
- Whether mTLS is required in the first customer deployment.
- Whether request signing is MVP or post-MVP.
- Whether HSM integration is required for payment network certification.
- How long to retain detailed request traces per tenant.

