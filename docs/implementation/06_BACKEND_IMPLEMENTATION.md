# Backend implementation plan

## 1. Purpose
Implement explicit Bidora operations with transactional integrity, not generic state CRUD.

## 2. Scope
Application composition, module ports, domain commands, PostgreSQL adapters, policy/error mapping and jobs.

## 3. Product specifications
[LLD](../product/09_LLD.md), [module catalogue](../product/07_MODULE_DEPENDENCIES.md), RULE-BID/DOC/IAM and SR-EVT.

## 4. Technical designs
[Backend architecture](../technical_design/08_BACKEND_ARCHITECTURE.md), [module contracts](../technical_design/09_BACKEND_MODULE_DESIGN.md), [API](../technical_design/10_API_CONTRACT_DESIGN.md).

## 5. Prerequisites
Approved command contract, actor policy, state transition and migration/repository test plan.

## 6. Required decisions
BID-ADR-007/011 for authority; 009/010 gate future sealed operations. Undefined policy returns a blocked operation, never permissive default.

## 7. Dependencies
Foundation → identity/policy/audit → provisioning → opportunity/document → compliance/response → review → package/evidence. Contract approval precedes every handler despite phase numbering.

## 8. Target files
Owner domain/application/adapters and module composition, API/worker entry points, tests. No direct imports of other modules' persistence.

## 9. Sequence
Write valid/invalid command tests; define application ports; implement domain transition; repository/transaction; audit/outbox; thin Fiber binding; safe DTO/errors; cancellation/shutdown; actual integration tests. Example FinalizeCandidate freezes exact source, requirement/evidence and response versions without approving them.

## 10. Domain/security
Resolve actor/tenant before DB access; recheck resource/state policy inside use case. Never trust browser status or include pricing in unauthorized DTO. Long network/scan/render work occurs outside SQL transaction.

## 11. Database/API/frontend
Conditional aggregate version update; same transaction audit/outbox; 409 stale conflict; UI refresh/compare rather than overwrite. 202 jobs have durable queryable state.

## 12. Tests
Approve/edit race, invalid SATISFIED, expired evidence, audit rollback, revoked job, duplicate mark-submitted, timeout after successful commit.

## 13. Acceptance
A command succeeds only when all guards hold and leaves exactly one authoritative transition under retry.

## 14. Done
Module ownership, contract tests, negative policy tests, observability and evidence reviewed.

## 15. Risks
Over-generalized workflow framework and cross-module repository shortcuts. Start approved bounded workflow, extend via versioned policies.

## 16. Evidence
Operation sequence, rule IDs, API/DB versions, executed test reports, trace and remaining policy blockers.

## 17. Exclusions
Full microservices, direct AI decisions, unrestricted status PATCH, future sealed handlers before review.
