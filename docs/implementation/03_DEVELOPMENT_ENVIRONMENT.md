# Historical development environment plan

> Historical proposal, not current setup instructions or an active approval gate. Use [Everyday development](../DEVELOPMENT.md) for working commands, runtime selection and validation. The proposed Make targets below were never implemented; current versions come from repository manifests. Security requirements remain applicable.

## 1. Purpose
Specify a reproducible local environment without creating it in this task.

## 2. Scope
Pinned toolchains, Compose profiles, synthetic data, configuration and safe startup/reset commands.

## 3. Product specifications
[Operations §§2–4](../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), [technology stack](../product/21_TECHNOLOGY_DECISIONS.md), M01.

## 4. Technical designs
[Deployment](../technical_design/21_DEPLOYMENT_ARCHITECTURE.md), [repository](../technical_design/02_REPOSITORY_ARCHITECTURE.md).

## 5. Prerequisites
Approved path/tool versions and a local container runtime; no production credentials/data.

## 6. Required decisions
BID-ADR-005/008 approve `web/`, `backend/`, Goose and npm. Exact Go/Fiber/Node/PostgreSQL pins still require compatibility/security verification; see the [candidate register](../tickets/10_FOUNDATION_DECISION_PACKET.md). Local service/configuration approval remains a separate gate.

## 7. Dependencies
Start minimal API/web/PostgreSQL foundation; add NATS/Redis/mail for identity/jobs and MinIO/scanner for document slice. AI/observability may be optional profiles.

## 8. Target files
Future README, Makefile, compose.yaml, safe .env.example, ignored local secret files and infrastructure/docker definitions.

## 9. Sequence
Pin runtimes/images; define validated config schema; document boot/migrate/check commands; create synthetic fixtures; test clean machine workflow; add safe health checks. Proposed targets: `make dev-up`, `make migrate`, `make check`, `make test-integration`, `make test-e2e`. They do not yet exist.

## 10. Domain/security
Separate DBs/roles even locally; no default tenant fallback. Secrets never committed or echoed. Explicit development-only reset command requires confirmation; ordinary startup never wipes volumes or auto-migrates.

## 11. Database/API/frontend
Control and tenant databases may share local PostgreSQL server; browser uses same-origin API proxy; backend configuration never enters public Next.js variables.

## 12. Tests
Fresh checkout boot, invalid config rejection, port/dependency failures, schema compatibility, graceful stop/restart and synthetic login when implemented.

## 13. Acceptance
Another engineer reproduces documented setup without hidden manual server edits; optional AI absence does not break deterministic workflows.

## 14. Done
Pinned versions, lockfiles, documented config names and successful clean-start evidence.

## 15. Risks
Laptop resource exhaustion from all services; profiles and bounded resources mitigate. Development TLS shortcuts must not leak into production.

## 16. Evidence
Runtime versions, commands, health outputs, startup duration and limitations; never capture secret values.

## 17. Exclusions
No dependency installation, Docker file creation or framework initialization as part of this documentation delivery.
