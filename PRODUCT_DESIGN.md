# Product Design

## Summary

This project is a Go-based multitenant API gateway for finance companies.  
It is built as first-party services (not Kong/APISIX/Tyk/KrakenD/Envoy runtime dependencies).

## MVP Scope

- Multitenant gateway runtime
- Protocol adapter model
- REST adapter
- ISO8583 adapter
- Control plane for tenant/config management
- Usage metering for billing
- Finance-grade security and audit logging

## Non-MVP Scope

- Payment collection and accounting
- Full API marketplace
- Full ESB replacement
- Full support for all protocol variants

## Core Product Requirements

- Strong tenant isolation in runtime and storage
- Protocol-agnostic routing and transformation flow
- Auditability for admin and runtime actions
- Sensitive data masking by default
- Replayable usage events for billing

## Primary Users

- API/integration engineers
- Platform and DevOps teams
- Security/compliance teams
- Billing/finance operations
