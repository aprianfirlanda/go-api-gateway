# Billing Design

## Billing Model

Billing is usage-based and event-driven.

- Meter each request attempt.
- Attribute usage to tenant, consumer, product, route, and protocol.
- Aggregate summaries by billing period.

## Data Requirements

- Usage events are append-only.
- Summaries are reproducible from raw usage events.
- Billing records exclude sensitive payload content.

## MVP Outputs

- Usage reporting APIs per tenant and period
- Invoice-ready summary exports for external finance systems

## Out of Scope

- Payment collection
- Tax/ledger accounting
- Dunning/subscription lifecycle automation
