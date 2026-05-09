# Transformation Design: Multitenant Protocol Gateway

## 1. Purpose

This document defines how the gateway transforms messages between protocols.

The transformation layer must support the first MVP flows:

- REST to REST.
- REST to ISO8583.
- ISO8583 to REST.

It must also support future flows:

- REST to SOAP/XML.
- SOAP/XML to REST.
- REST to gRPC.
- gRPC to REST.
- GraphQL to REST.
- Webhook to REST.
- Message queue event to REST.
- File record to REST.
- Proprietary TCP message to REST or ISO8583.

## 2. Design Goals

- Keep transformation logic protocol-neutral.
- Avoid hard-coded direct pairs like only REST to ISO8583.
- Use a canonical message model between protocol adapters.
- Make templates tenant-owned and versioned.
- Validate templates before publishing.
- Support dry-run testing.
- Mask sensitive values by default.
- Make transformation errors easy to debug without leaking sensitive data.

## 3. Core Concept

Each protocol adapter converts native protocol payloads into a canonical message.

The transformation engine maps one canonical message shape into another canonical message shape.

The target protocol adapter converts the transformed canonical message into the target native protocol.

```text
Inbound Protocol Payload
        |
        v
Inbound Protocol Adapter
        |
        v
Canonical Message
        |
        v
Transformation Template
        |
        v
Canonical Message
        |
        v
Outbound Protocol Adapter
        |
        v
Outbound Protocol Payload
```

## 4. Canonical Message Model

The canonical message is the internal representation used by the gateway.

```go
type CanonicalMessage struct {
    TenantID       string
    ConsumerID     string
    APIProductID   string
    RouteID        string
    SourceProtocol string
    TargetProtocol string
    Operation      string
    Headers        map[string]string
    Fields         map[string]any
    Metadata       map[string]any
    RawRef         string
    SensitiveKeys  []string
}
```

Field usage:

- `TenantID`: resolved tenant.
- `ConsumerID`: resolved consumer application.
- `APIProductID`: matched API product.
- `RouteID`: matched route.
- `SourceProtocol`: inbound protocol, such as `rest` or `iso8583`.
- `TargetProtocol`: outbound protocol, such as `iso8583` or `soap_xml`.
- `Operation`: business operation, such as `card_authorization`.
- `Headers`: normalized transport or protocol metadata.
- `Fields`: business payload fields.
- `Metadata`: non-business protocol metadata.
- `RawRef`: optional reference to a raw payload stored outside logs and billing records.
- `SensitiveKeys`: field names that must be masked in logs and errors.

Example:

```json
{
  "tenantId": "tenant_bank_a",
  "consumerId": "mobile_app",
  "apiProductId": "card_authorization",
  "routeId": "purchase_auth",
  "sourceProtocol": "rest",
  "targetProtocol": "iso8583",
  "operation": "purchase_authorization",
  "headers": {
    "requestId": "req_01HX000001"
  },
  "fields": {
    "transactionType": "purchase",
    "pan": "4111111111111111",
    "amount": 10000,
    "currency": "IDR",
    "terminalId": "ATM00101"
  },
  "metadata": {},
  "sensitiveKeys": [
    "pan"
  ]
}
```

## 5. Transformation Template Model

Transformation templates are tenant-owned, versioned, and immutable after publishing.

Recommended fields:

```text
id
tenantId
apiProductId
name
sourceProtocol
targetProtocol
version
templateBody
status
createdBy
publishedAt
createdAt
updatedAt
```

Status values:

- `draft`
- `published`
- `archived`
- `disabled`

Only `published` templates can be used by active routes.

## 6. Template Structure

Recommended template shape:

```yaml
name: card-authorization-rest-to-iso8583
sourceProtocol: rest
targetProtocol: iso8583
operation: purchase_authorization
request:
  fields:
    "2": "$.fields.pan"
    "3": "'000000'"
    "4": "formatAmount($.fields.amount)"
    "7": "nowMMddHHmmss()"
    "11": "generateStan()"
    "41": "$.fields.terminalId"
    "49": "currencyNumeric($.fields.currency)"
  sensitive:
    - "2"
response:
  fields:
    "responseCode": "$.fields.39"
    "authorizationCode": "$.fields.38"
    "stan": "$.fields.11"
    "rrn": "$.fields.37"
  mappings:
    status:
      source: "$.fields.39"
      values:
        "00": "approved"
        default: "declined"
```

Sections:

- `request`: transformation applied before upstream call.
- `response`: transformation applied after upstream response.
- `fields`: target field mapping.
- `sensitive`: target fields requiring masking.
- `mappings`: value translation rules.

## 7. Expression Language

MVP should use a small expression language instead of arbitrary scripting.

Supported expressions:

```text
$.fields.amount
$.headers.requestId
$.metadata.source
'static_value'
formatAmount($.fields.amount)
currencyNumeric($.fields.currency)
nowMMddHHmmss()
generateStan()
maskPan($.fields.pan)
coalesce($.fields.rrn, generateRrn())
```

Do not allow arbitrary user-provided code execution in MVP.

Reasons:

- Finance workloads require predictable execution.
- Arbitrary scripting increases security risk.
- Arbitrary scripting complicates performance limits.
- Arbitrary scripting is harder to audit.

## 8. Built-In Functions

MVP functions:

```text
formatAmount(value)
currencyNumeric(value)
currencyAlpha(value)
nowMMddHHmmss()
nowISO8601()
generateStan()
generateRrn()
maskPan(value)
left(value, count)
right(value, count)
padLeft(value, length, char)
padRight(value, length, char)
coalesce(value1, value2)
toString(value)
toInt(value)
```

Post-MVP functions:

```text
hash(value)
hmac(value, secretRef)
encrypt(value, keyRef)
decrypt(value, keyRef)
lookup(table, key)
```

Crypto-related functions should require explicit security review.

## 9. REST Mapping

REST adapter input:

- HTTP method.
- Path.
- Query parameters.
- Headers.
- JSON body.

REST canonical mapping:

```json
{
  "headers": {
    "method": "POST",
    "path": "/cards/authorization",
    "contentType": "application/json"
  },
  "fields": {
    "transactionType": "purchase",
    "amount": 10000,
    "currency": "IDR"
  }
}
```

REST output mapping:

```json
{
  "status": "approved",
  "responseCode": "00",
  "stan": "123456",
  "rrn": "654321123456"
}
```

## 10. ISO8583 Mapping

ISO8583 adapter input:

- Network length header.
- MTI.
- Bitmap.
- Data elements.

ISO8583 canonical mapping:

```json
{
  "headers": {
    "mti": "0100"
  },
  "fields": {
    "2": "4111111111111111",
    "3": "000000",
    "4": "000000010000",
    "7": "0508123015",
    "11": "123456",
    "37": "654321123456",
    "41": "ATM00101",
    "49": "360"
  },
  "sensitiveKeys": [
    "2"
  ]
}
```

REST to ISO8583 transformation output:

```json
{
  "headers": {
    "mti": "0100"
  },
  "fields": {
    "2": "4111111111111111",
    "3": "000000",
    "4": "000000010000",
    "7": "0508123015",
    "11": "123456",
    "41": "ATM00101",
    "49": "360"
  }
}
```

ISO8583 response mapping:

```json
{
  "headers": {
    "mti": "0110"
  },
  "fields": {
    "11": "123456",
    "37": "654321123456",
    "38": "A12345",
    "39": "00"
  }
}
```

## 11. ISO8583 Profile Dependency

ISO8583 transformation depends on a profile.

The profile defines:

- Field type.
- Field length.
- Fixed or variable length.
- Encoding.
- Bitmap behavior.
- Sensitive fields.
- Required fields by MTI.

Template validation must check that referenced ISO8583 fields exist in the active profile.

Example validation errors:

```json
{
  "errors": [
    {
      "field": "request.fields.130",
      "message": "ISO8583 field 130 is not defined in profile default-switch-profile"
    },
    {
      "field": "request.fields.4",
      "message": "field 4 must produce a 12-digit numeric amount"
    }
  ]
}
```

## 12. SOAP/XML Mapping

SOAP/XML adapter input:

- HTTP headers.
- SOAP envelope.
- XML namespaces.
- SOAP body.

SOAP/XML to canonical mapping example:

```yaml
sourceProtocol: soap_xml
targetProtocol: rest
request:
  fields:
    accountNumber: "xml($.Envelope.Body.AccountInquiry.accountNumber)"
    channelId: "xml($.Envelope.Body.AccountInquiry.channelId)"
response:
  fields:
    status: "$.fields.status"
    accountName: "$.fields.accountName"
    availableBalance: "$.fields.availableBalance"
```

REST to SOAP/XML output template:

```yaml
sourceProtocol: rest
targetProtocol: soap_xml
request:
  envelope:
    version: "1.1"
    namespaces:
      soapenv: "http://schemas.xmlsoap.org/soap/envelope/"
      bank: "http://example.com/bank"
  body:
    bank:AccountInquiry:
      bank:accountNumber: "$.fields.accountNumber"
      bank:channelId: "$.fields.channelId"
```

## 13. gRPC Mapping

gRPC support should use protobuf descriptors or generated clients.

Canonical mapping should include:

- Service name.
- Method name.
- Metadata.
- Request message fields.
- Response message fields.

Example:

```json
{
  "metadata": {
    "grpcService": "bank.AccountService",
    "grpcMethod": "GetAccount"
  },
  "fields": {
    "accountNumber": "1234567890",
    "channelId": "MOBILE"
  }
}
```

gRPC should be post-MVP unless needed by the first customer.

## 14. File and Batch Mapping

File adapters should convert each valid record into one canonical message.

Supported future formats:

- CSV.
- Fixed-width.
- JSON Lines.

CSV example:

```csv
account_number,amount,currency
1234567890,10000,IDR
```

Canonical record:

```json
{
  "fields": {
    "accountNumber": "1234567890",
    "amount": 10000,
    "currency": "IDR"
  },
  "metadata": {
    "batchId": "batch_01HX000001",
    "recordNumber": 1
  }
}
```

Batch behavior:

- Track batch ID.
- Track record number.
- Support partial failure.
- Produce per-record usage events if records are billable.

## 15. Message Queue Mapping

Message queue adapters should convert events into canonical messages.

Supported future systems:

- Kafka.
- RabbitMQ.
- NATS.
- Cloud queue services.

Canonical metadata:

```json
{
  "metadata": {
    "topic": "transaction-events",
    "partition": 1,
    "offset": 1024,
    "messageId": "msg_01HX000001"
  }
}
```

Queue behavior:

- Support at-least-once delivery.
- Support idempotency keys.
- Support retry.
- Support dead-letter queue.

## 16. Template Validation

Templates must be validated before publishing.

Validation rules:

- Source protocol is supported.
- Target protocol is supported.
- Referenced route or API product belongs to tenant.
- Referenced ISO8583 fields exist in selected profile.
- Required target fields are mapped.
- Sensitive fields are marked.
- Functions exist.
- Function arguments are valid.
- Static values match target field type and length.
- XML namespaces are valid for SOAP/XML templates.
- Template does not use disabled functions.

Validation response:

```json
{
  "valid": false,
  "errors": [
    {
      "field": "request.fields.4",
      "message": "formatAmount output must be 12 digits"
    }
  ],
  "warnings": [
    {
      "field": "request.fields.2",
      "message": "PAN field should be marked sensitive"
    }
  ]
}
```

## 17. Dry-Run Testing

Dry-run testing allows users to test a template without publishing it.

Input:

```json
{
  "direction": "request",
  "sourceProtocol": "rest",
  "targetProtocol": "iso8583",
  "input": {
    "fields": {
      "pan": "4111111111111111",
      "amount": 10000,
      "currency": "IDR",
      "terminalId": "ATM00101"
    }
  }
}
```

Output:

```json
{
  "output": {
    "headers": {
      "mti": "0100"
    },
    "fields": {
      "2": "411111******1111",
      "3": "000000",
      "4": "000000010000",
      "41": "ATM00101",
      "49": "360"
    }
  },
  "masked": true,
  "warnings": []
}
```

Dry-run output should mask sensitive values by default.

## 18. Versioning and Publishing

Rules:

- Draft templates are editable.
- Publishing creates an immutable version.
- Active routes reference a specific published template version.
- New template versions do not automatically affect active routes unless configured.
- Rollback means pointing the route back to an older published version.

Recommended publish flow:

1. Create draft template.
2. Validate draft.
3. Dry-run with sample payloads.
4. Publish version.
5. Attach published version to route.
6. Publish route config.
7. Gateway reloads config.

## 19. Error Handling

Transformation errors should be structured.

Error fields:

```text
code
message
templateId
templateVersion
direction
field
requestId
tenantId
routeId
```

Common errors:

```text
template_not_found
template_not_published
field_not_found
function_not_found
invalid_function_argument
invalid_target_type
required_field_missing
iso8583_field_not_defined
xml_path_not_found
```

Public error responses must not expose sensitive payload values.

## 20. Sensitive Data Handling

Sensitive fields:

- PAN.
- CVV.
- PIN block.
- Account number.
- National ID.
- Customer name, depending on tenant policy.

Rules:

- Full PAN must not be logged.
- CVV must not be stored or logged.
- PIN block must not be logged.
- Dry-run output masks sensitive values by default.
- Transformation errors must include field names, not sensitive values.
- Billing events must not include payload values.

Masking examples:

```text
4111111111111111 -> 411111******1111
1234567890 -> ******7890
```

## 21. Performance Requirements

MVP targets:

- Template lookup should be in-memory during request processing.
- Template execution should avoid database calls.
- Transformation overhead should stay below 10 ms for normal JSON and ISO8583 payloads.
- Large file and batch transformations should run outside the synchronous HTTP request path.

## 22. Testing Strategy

Unit tests:

- Field path resolution.
- Static value mapping.
- Function execution.
- Missing field behavior.
- Type conversion.
- Sensitive field masking.
- ISO8583 field validation.
- XML path resolution.
- Template validation.

Integration tests:

- REST to REST.
- REST to ISO8583.
- ISO8583 to REST.
- REST to SOAP/XML.
- Dry-run template test.
- Template publish and route attach.

Test fixtures:

- Sample REST authorization request.
- Sample ISO8583 authorization request.
- Sample ISO8583 authorization response.
- Sample SOAP account inquiry request.
- Sample CSV batch file.

## 23. Open Decisions

Open decisions:

- Whether template bodies should be stored as JSONB only or allow YAML input converted to JSONB.
- Whether the expression language should be custom or use a restricted existing expression library.
- Whether transformation functions should be globally available or tenant-enabled.
- Whether canonical schemas should be explicitly versioned per API product.
- Whether dry-run tests should be stored as reusable test cases.
