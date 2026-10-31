# DevOps and deployment implementation

## 1. Purpose
Promote reproducible artifacts and recover safely without provider-specific assumptions.

## 2. Scope
Local/staging/production topology, CI/CD, secrets/networking, migration rollout, feature flags, backup and incidents.

## 3. Product specifications
[Operations](../product/18_DEVOPS_DEPLOYMENT_OPERATIONS.md), OPS-001–006 and M01/M25.

## 4. Technical designs
[Deployment](../technical_design/21_DEPLOYMENT_ARCHITECTURE.md), [CI/CD](../technical_design/22_DEVOPS_CICD_DESIGN.md), [DR](../technical_design/26_DISASTER_RECOVERY.md).

## 5. Prerequisites
Approved hosting/region/CI/secrets/backup owner; cost and HA requirements; supported image/tool versions.

## 6. Required decisions
BID-ADR-013 production provider and recovery targets, 008 migration tool, 016 proxy/cookie trust.

## 7. Dependencies
Reproducible foundation and tests before images; migration compatibility and restore rehearsal before live data; acceptance before feature exposure.

## 8. Target files
Future infrastructure/docker, approved IaC location, runbooks, monitoring, Make targets and selected CI configuration.

## 9. Sequence
Pin/build non-root artifacts; scan/SBOM; provision isolated staging; inject secret references; TLS/private networks; migrate canary; deploy same digests; smoke/regression; monitor; bounded rollout; authorized feature activation; retain release evidence.

## 10. Domain/security
API cannot provision DBs; deployment and migrator privileges separated. No production secrets in PR jobs. Internal DB/broker/AI endpoints not public. Logs/health do not reveal locators.

## 11. Database/API/frontend
Expand-contract supports old/new binaries; same-origin web/API routing; private storage versioning; health semantics distinguish process liveness from required readiness.

## 12. Tests
Container startup/shutdown, network denial, secret rotation, migration failure pause, incompatible rollback rejection, backup restore and user E2E after cutover.

## 13. Acceptance
Staging artifact equals production digest; rollback/forward-fix rehearsed; approved RPO/RTO measured; on-call receives actionable alerts.

## 14. Done
Release approvals, provenance, deployment/migration history, runbook and recovery evidence.

## 15. Risks
Treating Compose restart as HA; destructive down migration; rebuilding after staging; avoid with explicit topology and immutable promotion.

## 16. Evidence
Image digests/SBOM, CI reports, migration ledger, timed restore, alert drill and signed deployment decision.

## 17. Exclusions
Selecting a cloud implicitly, Kubernetes by default, writing infrastructure in this task, deployment without authorized release.
