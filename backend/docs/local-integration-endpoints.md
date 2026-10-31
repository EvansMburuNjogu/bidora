# Local synthetic TLS integration endpoints

Status: **configured local test dependencies; PostgreSQL and Redis were already running when the authorized AUTH-002 verification began and their pre-run state was preserved; not production or route activation evidence**.

The user authorized creation on 2026-10-18. The services are isolated under `~/.local/share/bidora/integration/`, loopback-only, synthetic-data-only, and stay within the approved 2 CPU / 2 GiB / 10 GiB budget.

- PostgreSQL 15.19 TLS/SCRAM: `localhost:55432`
- Redis 8.6.1 TLS-only: `localhost:56379`
- Private environment file: `~/.local/share/bidora/integration/test-endpoints.env` (mode 0600)
- CA/server certificate lifetime: seven days; recreate rather than extending silently.
- PostgreSQL databases: one control and two tenant-isolation targets, each with separate migrator/runtime credentials.
- Redis: default user disabled, authenticated `bidora` user restricted to `bidora:*`, 64 MiB/no-eviction, persistence disabled.
- Synthetic Platform Owner credentials: `~/.local/share/bidora/integration/local-platform-admin.env` (mode 0600; never print or commit). The seeded identity is verified, active, TOTP-enrolled and authoritatively assigned `PLATFORM_OWNER`; browser acceptance remains pending.

Load credentials only into the command that needs them:

```sh
source "$HOME/.local/share/bidora/integration/test-endpoints.env"
```

Never print, commit or copy the file into the repository. The endpoints do not run migrations at application startup.

Manual shutdown:

```sh
"$HOME/.local/share/bidora/integration/postgresql-15.19-tls/bin/pg_ctl" \
  -D "$HOME/.local/share/bidora/integration/postgres-data" -m fast -w stop
kill "$(cat "$HOME/.local/share/bidora/integration/redis/redis.pid")"
```

Acceptance evidence: verified-TLS connections and the authorized 2026-10-20 baseline to control schema 13, tenant-common schema 5 and Tenderpreneur schema 26, followed by later additive upgrades through control 29, tenant-common 9 and Tenderpreneur 34 in both tenant databases. V6 applied Tenderpreneur migrations 33–34 through reviewed migrator credentials, verified compatibility 34 in both tenants and advanced compatible control projections before shutdown. V8 then exercised migrations 20–23 through the already-migrated synthetic tenant, including exact submission-proof scanning and one-winner confirmation against both isolated tenant targets. V9 exercised migrations 24–25 through outcome-proof scanning, correction races, archive blockers, immutable closure and tenant-B non-observation. During V10 preflight, the reviewed version-1 runtime grants were reapplied successfully to the original control and both tenant fixtures (68/61/61 tables). The full integration run then stopped with six full-pilot compositions blocked by stale mandatory document-processing/TLS NATS fixture configuration; this is not a passed restart, provider or release gate. Earlier evidence also covers cross-tenant connection denial, runtime DDL denial, plaintext PostgreSQL/Redis rejection and Redis out-of-scope key denial. The centralized live capture integration test additionally verifies production-pool TLS, compatibility, atomic commit/rollback, replay/conflict, same-key concurrency, stale assignment leases and tenant-B non-observation. Run it with:

```sh
source "$HOME/.local/share/bidora/integration/test-endpoints.env"
cd backend
go test -count=1 -race -v ./tests/integration
```

AUTH-002 subsequently applied the independent control-auth target 29 and passed race-enabled registration/verification/resend evidence against TLS PostgreSQL, TLS Redis and in-process authenticated STARTTLS. It verified exact personal-name persistence, duplicate-name privacy, replacement-token invalidation, atomic consumption/replay denial, unknown-target concealment and fail-closed resend limits. External SMTP and browser acceptance remain pending.\n\nThe centralized live Redis integration tests verify production-client TLS/authentication, identity TTL/denial/reset, network-counter retention, 40-way atomic reservation limits, single-use finish, ACL namespace denial and fail-closed connection refusal. The live identity integration additionally composes real Argon2id, PostgreSQL credential/session/audit adapters and Redis attempt controls through login, session validation/touch and logout. It verifies stale-password rehash, generic invalid credentials, session/audit commits and revoked-session rejection. A temporary-listener HTTP integration test now verifies the complete synthetic login → CSRF/membership-authorized capture → replay → authorization-change races → logout flow and post-logout rejection using all real adapters. After login it also verifies the approved CSRF-only authenticated bootstrap returns the cookie/session-bound token through an exact-Origin, no-store response. Its authorization-change evidence includes active DENY precedence, stale context rejection, assignment-generation routing failure, and successful capture after the synthetic assignment is restored. It then closes only that application runtime's Redis client: an existing PostgreSQL-backed session can still capture and log out, while a new login returns generic `503`, emits no cookies and persists no session. The flow uses the live control assignment reader and exact server-owned store catalog rather than a fixed/caller-selected tenant pool. The live flow uses production `pilot.Open` and `pilot.Serve`, including pool creation, Redis/PostgreSQL probes, schema-compatibility checks, readiness transition, listener cancellation/drain, the `pilot.Build` composition root and its `httpapi.NewPilot` route factory. Composition receives the live pools, Redis TLS configuration, Origin, proxy prefix and exact tenant assignment explicitly; it does not discover ambient DSNs or secrets. It does not switch `cmd/api` from its health-only constructor. A separate live bootstrap-recovery test verifies denial for unavailable Redis authentication, refused control/tenant PostgreSQL and stale assignment generation, then successful readiness after valid targets are supplied again. No service was stopped for that test; actual daemon restart and in-process reconnect evidence remain separate. The control migrator subsequently applied Google external-identity migration 8, and the fixture runtime role received only `SELECT, INSERT, UPDATE` on the new table. Live Google persistence evidence verifies stable-subject signup/sign-in, zero membership, canonical-email preservation and denial of email-only linking; this does not supply OIDC callback, Redis transaction-state or JWKS evidence.
