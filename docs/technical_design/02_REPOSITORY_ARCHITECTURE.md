# Repository architecture

Status: root choice approved by BID-ADR-005; detailed runtime tree remains a design, not implemented files. Sources: [LLD §1](../product/09_LLD.md), [source agent guide](../product/23_AGENTS.md). Execution: [foundation](../implementation/04_FOUNDATION_IMPLEMENTATION.md).

## Observed versus target

The user approved retaining the existing empty `backend/` and `web/` roots on 2026-10-17. `frontend/` is absent and must not be created as a second application. `<frontend-root>` anywhere in this collection now resolves to `web/`, not a literal directory. npm with one `web/package-lock.json` is approved; exact runtime pins and runnable proof remain gated by the [toolchain packet](../tickets/10_FOUNDATION_DECISION_PACKET.md).

```text
AGENTS.md
backend/
  cmd/{api,worker,migrate}/
  internal/
    platform/{identity,tenancy,subscription}/
    shared/{authz,audit,document,eventing,workflow,notification}/
    tenderpreneur/{profile,opportunity,bid,readiness}/
    procurement/{tender,publication,opening,evaluation,award}/  # later
    exchange/submission/                                     # later
    infrastructure/{config,postgres,telemetry}/
  migrations/{control,tenant_common,tenant_tenderpreneur}/
  queries/                     # owned subfolders, sqlc inputs
  api/{openapi,events}/         # generated/validated artifacts later
  tests/{integration,contract}/
web/
  src/{app,features,components,contracts,lib}/
  tests/{component,e2e}/
ai/                            # created only for approved AI slice
infrastructure/{docker,monitoring,runbooks}/
docs/{product,technical_design,implementation}/
```

## Ownership and dependency direction

Each backend feature contains `domain`, `application`, `adapters`, and `module.go`. Domain does not import SQL, Fiber, other modules' adapters or cloud SDKs. Application owns ports. Composition supplies implementations. A common technical package is not a home for shared mutable business entities. Only owner-module queries touch its tables; cross-module use cases compose public ports inside an explicitly shared same-database unit of work where required.

Frontend feature folders own domain-specific orchestration; `components/ui` contains reusable primitives. Generated contracts are not hand-edited. Database rows, domain aggregates and wire DTOs are separate types.

## Repository controls

Future CI checks import direction, generated-file drift, schema compatibility, formatting and lockfiles. Pin toolchain and image digests after compatibility tests; this document selects no unverifiable 'latest' versions. Ignore credentials, local volumes, dependency/build outputs and `.DS_Store`; do not delete existing unrelated files in a documentation task.

Keep authoritative definitions in schema, contract and decision documents. Combined product specification is derivative; do not edit it independently. Root AGENTS links actual `docs/product/` paths rather than nonexistent `docs/specifications/`.

## Verification

A clean checkout must have one documented boot/check workflow. A proposed new root or dependency requires an ADR/ticket. Confirm runtime roots are unchanged during documentation-only tasks; actual automated validation is recorded in [quality report](QUALITY_REPORT.md).
