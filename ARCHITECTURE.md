# Architecture

## Components

- **Gateway data plane**: processes inbound/outbound traffic.
- **Control plane API**: manages tenants, routes, credentials, policy, and adapter config.
- **PostgreSQL**: system-of-record configuration and usage/audit data.
- **Redis**: runtime state/cache where enabled.
- **Billing worker**: aggregates usage events into billing summaries.

## High-Level Topology

```text
Admin/Operator -> Control Plane API -> PostgreSQL
                                      -> Redis (optional runtime state)

Client/Partner -> Gateway Data Plane -> Upstreams (REST/ISO8583/others)
                                    -> Usage Events / Audit Logs
```

## Deployment Notes

- Run data plane and control plane as separate services.
- Prefer stateless gateway instances with horizontal scaling.
- Keep control-plane writes decoupled from runtime request path.
