# TENDERPRENEUR-MEMBERSHIP-001 — invitation-based membership entry

Status: **VERIFIED_TECHNICALLY — bounded synthetic control-plane journey**

## Outcome

A verified synthetic account that has deliberately selected `TENDERPRENEUR` can enter an existing active Tenderpreneur organization through the approved invitation flow. The journey does not create an organization, tenant database, assignment or subscription.

## Verified journey

The focused centralized integration journey uses the existing isolated PostgreSQL 15.19 and Redis 8.6.1 TLS fixtures and an already-existing active synthetic Tenderpreneur tenant:

1. create synthetic verified manager and invitee identities;
2. give only the manager the existing tenant's membership-management capability;
3. log in the invitee and select `TENDERPRENEUR` through the Origin/CSRF/idempotency boundary;
4. confirm `/v1/me` has no portal and requires `ESTABLISH_TENDERPRENEUR_MEMBERSHIP`;
5. issue an opaque, hashed, expiring `BID_MANAGER` invitation through the production application/persistence boundary;
6. accept it once as the invited identity;
7. confirm one current active membership exists;
8. confirm `/v1/me` now resolves exactly `TENDERPRENEUR` at `/tenderpreneur` with no remaining required action;
9. confirm the control tenant count is unchanged.

The raw invitation token and synthetic credentials are held only in process and are not logged or persisted as plaintext by the journey.

## Evidence

```text
go test -count=1 -run '^TestLiveTenderpreneurInvitationMembershipEnablesPortalWithoutCreatingOrganization$' ./tests/integration
```

Result: PASS in 1.476 seconds.

An initial attempt to reuse the broad full-pilot invitation/capture journey failed before execution with `pilot_deployment_open_failed`, consistent with the separately recorded NATS composed-runtime gate. The focused control-plane journey was then used so this membership evidence did not weaken or bypass full-pilot readiness.

This is technical integration evidence, not browser/UAT/deployment/provider acceptance. The existing loopback API and web were not restarted.
