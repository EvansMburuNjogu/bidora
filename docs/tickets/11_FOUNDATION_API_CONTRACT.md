# BID-API-001 — Foundation API contract review packet

Status: **Original proposal; health-only implementation now exists**. See the [published contract](../../backend/api/openapi/health.json) and [executed checks](12_HEALTH_SLICE_EVIDENCE.md) for the current bounded slice. Original unexecuted-check statements below describe the proposal-time snapshot, not the subsequent implementation. Ticket [BID-API-001](backend_contracts/BID-API-001.md) remains Blocked by ARC-002 closure and contract-owner approval. Preparing this packet does not bypass those gates.

Sources: [SRD API-001–008 and OPS-001](../product/04_SRD.md), [API conventions](../technical_design/10_API_CONTRACT_DESIGN.md), [foundation plan](../implementation/04_FOUNDATION_IMPLEMENTATION.md), [toolchain decisions](10_FOUNDATION_DECISION_PACKET.md).

## 1. Bounded first operation

A user opens the foundation smoke page and sees the real Go service's readiness, or an honest unavailable state. No users, tenants, documents or business decisions are created. The API must not expose private infrastructure details.

Business APIs retain `/v1`. The explicit operational exceptions `/health/live` and `/health/ready` follow the existing foundation plan; they do not replace the future authenticated `/v1/platform/service-health` operation.

## 2. Endpoint cards

| Contract field | GET /health/live | GET /health/ready |
|---|---|---|
| Purpose | Process can answer requests | Service can accept work for its configured profile |
| Authentication | No session or tenant context required; minimal payload only | Same; detailed dependency diagnostics remain private |
| Request | No body, no query parameters, no tenant selector | Same |
| Success | 200, `data.status = live` | 200, `data.status = ready` |
| Dependency failure | Does not turn a live process unhealthy just because PostgreSQL/broker is unavailable | 503 `SERVICE_NOT_READY`, generic message and `retryable = true` |
| Drain/startup | Process may remain live during drain; no guarantee of successful business traffic | 503 until initialized, and immediately upon drain |
| Side effects | No DB mutations, tenant provisioning, audit business event or provider calls | Read-only checks against required initialized resources; no migrations or lazy tenant provisioning |
| Replay/concurrency | Safe repeated GET; no Idempotency-Key or If-Match | Same |
| Caching | `Cache-Control: no-store` | Same |
| Correlation | Server-generated UUID `X-Request-ID`, equal to response `trace_id` | Same |
| Tests | Process remains live during required-dependency outage; no secret fields | Required outage/drain blocks readiness; optional absent service does not |

Proposed invalid request behavior: unexpected body or query parameters return 400 `INVALID_REQUEST`; unsupported methods return 405 with `Allow: GET`. HEAD/OPTIONS are not implicitly promised; approve any probe/edge requirements before adding them. Health routes do not consume cookies for authorization and must not mint sessions. No CORS relaxation is implied.

### Readiness semantics

- Configuration validation occurs before serving. Invalid configuration causes bounded, safely logged startup failure, not a falsely ready server.
- A configured required PostgreSQL dependency must be initialized and respond to a bounded read-only check. Required dependencies are explicit in the approved profile; empty configuration must not silently mean healthy.
- Optional services are not checked unless the selected profile requires them. Manual workflow does not depend on AI being available.
- Do not scan every tenant database in a global readiness probe. Per-tenant routing failures have their own request handling and private operational telemetry.
- During drain, stop accepting new work through readiness first, then allow bounded in-flight completion. A disconnected client or expired probe deadline cancels its work.
- Dependency probe timeout, total request budget, polling interval, concurrency cap and thresholds remain configurable **pending ARC-007/FND-003 approval**. This packet invents no production SLO.
- No dependency names, hostnames, connection strings, stack traces, version banners or topology are returned. Internal diagnosis uses safe correlation and access-controlled telemetry.

## 3. Proposed OpenAPI 3.1 artifact

The JSON below is the proposed contract artifact embedded in documentation, not an application file. After approval, API-001 may publish it under the authorized `backend/api/openapi/` path and add reviewed tooling. OpenAPI lint and full JSON Schema example validation have **not** run: the relevant validators are not installed and this task installs nothing.

`additionalProperties: false` is intentional for the minimal health/error serialization contract. Later output additions require an explicit compatibility review and schema update; consumers must not be assumed forward-compatible without tests.

```json
{
  "openapi": "3.1.0",
  "info": {"title": "Bidora Foundation Health API", "version": "0.1.0-draft"},
  "security": [],
  "paths": {
    "/health/live": {
      "get": {
        "operationId": "getLiveness",
        "description": "Process responsiveness only; no tenant or dependency details.",
        "responses": {
          "200": {
            "description": "Process is live",
            "headers": {
              "Cache-Control": {"$ref": "#/components/headers/NoStore"},
              "X-Request-ID": {"$ref": "#/components/headers/RequestId"}
            },
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LiveResponse"}}}
          },
          "400": {"$ref": "#/components/responses/InvalidRequest"},
          "405": {"$ref": "#/components/responses/MethodNotAllowed"},
          "500": {"$ref": "#/components/responses/InternalError"}
        }
      }
    },
    "/health/ready": {
      "get": {
        "operationId": "getReadiness",
        "description": "Required configured resources initialized and usable; false during drain.",
        "responses": {
          "200": {
            "description": "Service is ready",
            "headers": {
              "Cache-Control": {"$ref": "#/components/headers/NoStore"},
              "X-Request-ID": {"$ref": "#/components/headers/RequestId"}
            },
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ReadyResponse"}}}
          },
          "400": {"$ref": "#/components/responses/InvalidRequest"},
          "405": {"$ref": "#/components/responses/MethodNotAllowed"},
          "500": {"$ref": "#/components/responses/InternalError"},
          "503": {
            "description": "Not ready; safe to retry after configured backoff",
            "headers": {
              "Cache-Control": {"$ref": "#/components/headers/NoStore"},
              "X-Request-ID": {"$ref": "#/components/headers/RequestId"}
            },
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ErrorResponse"}}}
          }
        }
      }
    }
  },
  "components": {
    "headers": {
      "NoStore": {"description": "Responses must not be cached", "schema": {"type": "string", "const": "no-store"}},
      "RequestId": {"description": "Server-generated request correlation UUID", "schema": {"type": "string", "format": "uuid"}}
    },
    "responses": {
      "MethodNotAllowed": {
        "description": "Unsupported method; route-level behavior also tested outside listed GET operations",
        "headers": {
          "Allow": {"schema": {"type": "string", "const": "GET"}},
          "Cache-Control": {"$ref": "#/components/headers/NoStore"},
          "X-Request-ID": {"$ref": "#/components/headers/RequestId"}
        },
        "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ErrorResponse"}}}
      },
      "InvalidRequest": {
        "description": "Malformed or unexpected health input",
        "headers": {
          "Cache-Control": {"$ref": "#/components/headers/NoStore"},
          "X-Request-ID": {"$ref": "#/components/headers/RequestId"}
        },
        "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ErrorResponse"}}}
      },
      "InternalError": {
        "description": "Unexpected failure with safe generic details",
        "headers": {
          "Cache-Control": {"$ref": "#/components/headers/NoStore"},
          "X-Request-ID": {"$ref": "#/components/headers/RequestId"}
        },
        "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ErrorResponse"}}}
      }
    },
    "schemas": {
      "Meta": {
        "type": "object", "additionalProperties": false, "required": ["trace_id"],
        "properties": {"trace_id": {"type": "string", "format": "uuid"}}
      },
      "LiveResponse": {
        "type": "object", "additionalProperties": false, "required": ["data", "meta"],
        "properties": {
          "data": {
            "type": "object", "additionalProperties": false, "required": ["status"],
            "properties": {"status": {"type": "string", "const": "live"}}
          },
          "meta": {"$ref": "#/components/schemas/Meta"}
        }
      },
      "ReadyResponse": {
        "type": "object", "additionalProperties": false, "required": ["data", "meta"],
        "properties": {
          "data": {
            "type": "object", "additionalProperties": false, "required": ["status"],
            "properties": {"status": {"type": "string", "const": "ready"}}
          },
          "meta": {"$ref": "#/components/schemas/Meta"}
        }
      },
      "FieldError": {
        "type": "object", "additionalProperties": false, "required": ["path", "code", "message"],
        "properties": {
          "path": {"type": "string"},
          "code": {"type": "string", "minLength": 1},
          "message": {"type": "string", "minLength": 1}
        }
      },
      "ErrorResponse": {
        "type": "object", "additionalProperties": false, "required": ["error"],
        "properties": {
          "error": {
            "type": "object", "additionalProperties": false,
            "required": ["code", "message", "field_errors", "trace_id", "retryable"],
            "properties": {
              "code": {"type": "string", "minLength": 1},
              "message": {"type": "string", "minLength": 1},
              "field_errors": {"type": "array", "items": {"$ref": "#/components/schemas/FieldError"}},
              "trace_id": {"type": "string", "format": "uuid"},
              "retryable": {"type": "boolean"}
            }
          }
        }
      }
    }
  }
}
```

Status/code/retryability correspondence and header/body correlation equality require semantic conformance tests in addition to shape validation. The generic ErrorResponse schema alone does not enforce these invariants.

## 4. Examples and safe error mapping

Synthetic readiness success:

```json
{"data":{"status":"ready"},"meta":{"trace_id":"019c0000-0000-7000-8000-000000000002"}}
```

Synthetic required-dependency failure (503):

```json
{"error":{"code":"SERVICE_NOT_READY","message":"The service is not ready to accept requests.","field_errors":[],"trace_id":"019c0000-0000-7000-8000-000000000002","retryable":true}}
```

| HTTP | Foundation code | Retryability | Rule |
|---|---|---|---|
| 400 | INVALID_REQUEST | false | Health input shape invalid; no raw rejected value echoed |
| 405 | METHOD_NOT_ALLOWED | false | Safe error envelope and `Allow: GET`; framework must not emit uncontrolled HTML |
| 500 | INTERNAL_ERROR | false | No blanket retry advice for future writes; health GET client may offer manual retry |
| 503 | SERVICE_NOT_READY | true | Readiness unavailable; generic message, no dependency locator |

A 503 does not certify a useful Retry-After duration. Omit that header unless an approved policy supplies a truthful value. All errors retain the same server-generated request identifier in header and body. Incoming request-ID values are ignored for this initial public health boundary; they cannot forge audit identity. W3C trace context and internal tracing are separate concerns, finalized with OBS-001.

## 5. General business-contract conventions to approve

These are scoped proposals to carry into subsequent API packets, not implemented business endpoints:

- JSON snake_case; required vs omitted vs nullable explicit in each schema. UUID strings validated as identifiers, never authority. RFC3339 UTC instants plus separate original timezone/text where business-relevant.
- Money uses decimal strings plus explicit currency; scale/range/rounding belong to approved business operation schemas, not a generic floating-point rule.
- Strict command inputs reject unknown properties. Define safe unknown-output behavior and enum expansion policy with client tests before compatibility claims.
- Stable cursor over approved sort/filter keys; proposed default 25/max 100 remains pending contract-owner approval. Cursor binding is integrity protected, not authorization.
- Authenticated tenant routes require current identity/context/capability/classification; 401/403/concealed-404 cannot be chosen ad hoc. Health has no implied identity bypass for business routes.
- 409 covers approved version/state/idempotency conflicts; 422 semantic validation; 429 rate/quota with truthful retry hints; 503 temporary dependency failure.
- Mutable operations require documented If-Match/expected-version rules. Proposed missing-precondition 428 remains unapproved until operation cards and clients agree.
- Critical transitions require scoped Idempotency-Key and request hash: same key/different input conflicts; replay reauthorizes; in-progress conflict is not final success. Key expiry never removes immutable business uniqueness.
- Numerical body/header/key/string/time limits need explicit owner-approved values. No universal limits are silently invented by this packet.

## 6. Frontend smoke states

| Observation | Required presentation |
|---|---|
| Request pending | Checking service; no fabricated healthy badge |
| Valid readiness 200 with `ready` | Ready, showing safe request correlation if useful |
| Readiness 503 | Service unavailable/not ready; retry action or approved bounded polling |
| Network failure/timeout | Cannot reach service; never convert to ready |
| Malformed JSON, wrong media type, unexpected status/body | Unexpected service response; safe generic error, no raw HTML/body injection |
| Liveness 200 while readiness 503 | Process responsive, service not ready; do not conflate the signals |

Go API base configuration stays server-only. Same-origin proxy details remain BID-ADR-016/ARC-006 approval; this packet does not approve credentialed CORS or production browser topology.

## 7. Required verification matrix

| Case | Expected assertion | Execution status |
|---|---|---|
| Missing `meta.trace_id`, unknown `data.dsn`, null `status`, string `retryable` | Schema rejection | Not run with JSON Schema validator |
| Wrong success status for endpoint | Shape/const rejection | Not run with validator |
| 200 body claiming readiness after required dependency failure | Runtime test fails | No handler exists |
| Database outage | live 200, ready 503; no credentials/hostnames | Not run |
| Optional AI absent | Readiness unaffected for manual profile | Not run |
| Client correlation spoof | Server generates own UUID; header/body match | Not run |
| Invalid query/body and unsupported method | Approved safe 400/405 and no HTML leak | Not run |
| Shutdown/drain | Readiness fails before shutdown; request bounded by approved limits | Not run |
| Add required request field/remove response field/change enum | Compatibility review gate fails | Tool not selected/run |
| Real browser→Go success/failure | Honest states, no mock success, keyboard retry | FND-002; not run |

## 8. Remaining approval and evidence gates

1. ARC-002 exact version/convention review; no automatic completion from tool selection.
2. Contract-owner acceptance of health shape, operational paths, method/input behavior, server correlation and error policy.
3. ARC-007/FND-003 dependency profile and numerical budgets; ARC-006/BID-ADR-016 browser topology.
4. Select and approve a pinned OpenAPI 3.1 linter, JSON Schema validator and client generation approach. No validator is currently installed in the inspected Python environment (`jsonschema` and `openapi_spec_validator` absent).
5. Run full schema/negative example and compatibility checks under an authorized tooling task. JSON parsing or reference traversal alone is not OpenAPI conformance.

Machine-readable draft available; ticket remains **Blocked**, no Ready/Done claim. Product sources and application directories are unchanged.
