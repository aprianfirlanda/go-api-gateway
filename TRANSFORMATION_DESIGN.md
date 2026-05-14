# Transformation Design

## Model

Use a canonical internal message between protocol adapters.

```text
Inbound payload -> Inbound adapter -> Canonical message
-> Transformation rules -> Canonical message
-> Outbound adapter -> Outbound payload
```

## MVP Flows

- REST -> REST
- REST -> ISO8583
- ISO8583 -> REST

## Rules

- Transformations are tenant-owned and versioned.
- Validation is required before activation.
- Runtime errors should be debuggable without exposing sensitive data.
- Mask sensitive financial fields in logs and audit records.
