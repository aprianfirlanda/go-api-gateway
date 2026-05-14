# AGENTS.md

## Project

Go-based multitenant API gateway for finance companies.

Implementation module: `backend/`.

## Required Reading

Before implementation work, read:

- `PRODUCT_DESIGN.md`
- `TECHNICAL_DESIGN.md`
- `ARCHITECTURE.md`
- `TECHNOLOGY_DECISIONS.md`
- `DATA_MODEL.md`
- `API_SPEC.md`
- `TRANSFORMATION_DESIGN.md`
- `SECURITY_DESIGN.md`
- `BILLING_DESIGN.md`

## Implementation Constraints

- Build in Go without gateway runtimes (Kong/APISIX/Tyk/KrakenD/Envoy).
- Keep protocol logic behind adapter interfaces.
- Keep tenant isolation explicit in models, repositories, and runtime path.
- Billing must use usage events, not logs.
- Never log PAN/CVV/PIN/API keys/tokens/secrets.
- Do implementation changes inside `backend/` unless explicitly asked otherwise.

## Locked Stack

Use `TECHNOLOGY_DECISIONS.md` as source of truth.
