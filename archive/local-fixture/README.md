# Retired local-fixture experiments — historical archive

Archived at the user's request to clean up and focus on application development. These are not application dependencies, active development tasks or completion evidence.

## Contents and integrity

`backend/internal/{localprofile,localcheck,localpostgres,localbootstrap,localbudget}`, their `backend/tests/` suites and `backend/cmd/local-check` were moved here **without changing their bytes**. `manifest.json` records every original path, archived path and SHA-256. The old `go.mod` and `go.sum` were copied as `.snapshot` files; they are not an executable module configuration.

The archive is outside the active backend Go module. `cd backend && go test ./...` intentionally tests the application, not this retired code. Historical test results remain historical; excluding these tests does not complete their tickets or validate the unfinished storage collector.

Original code references in older tickets and `backend/docs/local-*.md` / `native-*.md` resolve by prepending `archive/local-fixture/` to their former repository paths. Those documents are retained for evidence, not as current run instructions.

## Do not reactivate by default

- Do not import this code into the application or run old local-check/bootstrap instructions.
- Do not expand the archive, finish its collector, or replace it with newly named fixture/supervisor tooling.
- Restoring any part requires explicit task-specific authorization and review. Restore only needed original paths; reconcile module changes rather than overwriting the active go.mod/go.sum with snapshots.
- No host installations, databases, stopped Docker profiles, credentials, agent configuration or usage ledgers were removed or altered by this cleanup.

Application development lives in `backend/`. See the root `AGENTS.md` anti-diversion rules and `backend/docs/application-development.md` for the current bounded work.
