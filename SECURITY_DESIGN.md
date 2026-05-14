# Security Design

## Principles

- Deny by default.
- Authenticate before route execution.
- Enforce tenant isolation in runtime and data access.
- Do not log secrets or sensitive financial data.
- Make sensitive config changes auditable.

## Sensitive Data Rules

Never log full values for:

- PAN
- CVV
- PIN/PIN block
- API keys
- access tokens
- secret material

## Runtime Controls

- TLS for external/intra-service traffic (by environment policy)
- Credential hashing and secure comparison
- Per-tenant authorization boundaries
- Rate limit/quota enforcement hooks
- Structured audit trail for admin actions

## Storage Controls

- Secret references preferred over plaintext secret values
- Least-privilege DB/service accounts
- Strict tenant filters in repositories
