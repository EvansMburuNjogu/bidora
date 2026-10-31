# TENDERPRENEUR-ACCOUNT-001 — Tenderpreneur account entry

Status: **VERIFIED_TECHNICALLY — synthetic control-plane journey**

## Outcome

The bounded identity/account runtime now composes the complete existing control-plane entry sequence for a synthetic Tenderpreneur account:

1. `POST /v1/auth/register` creates an unverified identity without authority.
2. `POST /v1/auth/verify-email` atomically verifies the issued challenge.
3. `POST /v1/auth/login` creates an authenticated browser session.
4. `GET /v1/me` returns `SELECT_ACCOUNT_TYPE` while the account is unassigned.
5. `POST /v1/me/account-type` records the deliberate, irreversible `TENDERPRENEUR` selection with CSRF/session/origin protection and idempotency.
6. `GET /v1/me` then returns `ESTABLISH_TENDERPRENEUR_MEMBERSHIP` and no portal when the account has no eligible membership.

The full pilot already mounted the account-type selection operation. This increment adds the previously omitted selection route to `NewIdentityAccountApp`, which is the smaller identity-only runtime used for bounded synthetic account journeys.

## Authority boundary

Registration, verification, authentication and account classification do not create:

- an organization or organization database;
- a tenant assignment;
- membership, role or capability;
- a subscription or trial;
- an invitation;
- access to `/tenderpreneur`.

The account reaches the Tenderpreneur portal only after a separately authorized current `ACTIVE` membership in an `ACTIVE` Tenderpreneur tenant exists. Headers and account type never grant that authority.

## Technical evidence

The centralized live integration journey now composes `BuildIdentityRuntime` against the existing isolated PostgreSQL 15.19 and Redis 8.6.1 TLS fixtures plus an in-process authenticated STARTTLS synthetic SMTP server. It creates a fresh synthetic identity, captures its verification token without logging it, verifies the identity, logs in, resolves `SELECT_ACCOUNT_TYPE`, selects `TENDERPRENEUR` through the authenticated Origin/CSRF/idempotency boundary, then proves:

- `/v1/me` reports `TENDERPRENEUR`;
- `portal` is `null`;
- the sole required action is `ESTABLISH_TENDERPRENEUR_MEMBERSHIP`;
- `/v1/me/memberships` returns an empty list.

Executed command:

```text
go test -count=1 -run '^TestLiveIdentityCompositionRegistrationVerificationAndVerifiedLogin$' ./tests/integration
```

Result: PASS in 2.584 seconds.

This is bounded technical verification, not browser/UAT/deployment/provider acceptance. The already-running older loopback binary was not restarted and web was not started. Registration remains available only when explicit SMTP registration configuration is supplied. Production HTTPS and existing session, CSRF, Origin, limiter, password, token and email-verification controls remain unchanged.
