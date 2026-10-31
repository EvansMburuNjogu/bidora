# Document measurement lease and retry decision packet

Status: **OPTION 1 APPROVED — SOURCE IMPLEMENTED_NOT_VERIFIED — WORKER UNMOUNTED**

Date: 2026-10-20

Tickets: BID-AI-003, BID-EVT-002

## Approved decision

On 2026-10-20 the requesting user proceeded with recommended Option 1 for a tender-analysis job left in `MEASURING` when its worker crashes, loses its database connection or is terminated before committing immutable measurement evidence.

The approved policy prevents both permanent stuck work and a stale worker committing after another worker has reclaimed the job. It does not activate the parser, change its approved limits or authorize provider execution.

## Existing constraints

- Parser wall timeout is 180 seconds.
- Parser CPU budget is 120,000 ms.
- The parser process must be externally contained and terminated as a process group.
- Measurement occurs outside SQL transactions.
- No credits are reserved and no provider is invoked until immutable measurement commits.
- The current `started_at` timestamp is not a sufficient fencing token: an expired worker could otherwise race a replacement worker.
- Retrying deterministic malformed/limit failures cannot produce an authoritative different result under the same parser, semantics and profile versions.

## Options

### Option 1 — fixed fenced lease with bounded transient retries (recommended)

- Use a non-renewable five-minute measurement lease.
- Every claim receives a new random UUID lease ID and database-calculated expiry.
- Release, cancellation and measurement commit require the exact current lease ID.
- A stale worker can finish local parsing but cannot alter job state or persist measurement evidence after its lease is replaced.
- Permit at most three total attempts: initial attempt plus two transient retries.
- Retry transient attempt 1 after 30 seconds and transient attempt 2 after 120 seconds.
- After the third transient failure or lease expiry, terminate as `MEASUREMENT_RETRY_EXHAUSTED`.
- Never retry deterministic document/parser-limit failures automatically.

This option fits the approved 180-second parser wall limit while leaving two minutes for process termination and worker/database cleanup before reclamation. No heartbeat is required for an operation that already has a hard wall limit.

### Option 2 — one attempt only

- Use the same five-minute fenced lease.
- Any failure or expiry becomes terminal.

This is simplest and minimizes repeated hostile-input work, but transient object-store, process-start or host interruption would require a new human-authorized operation that is not currently defined.

### Option 3 — renewable heartbeat lease

- Renew a shorter lease while parsing and reclaim after missed heartbeats.
- Retain bounded retries.

This adds heartbeat writes, renewal races and more failure states without extending the approved parser wall timeout. It is not recommended for the pilot.

## Approved Option 1 details

### Lease and fencing

- Lease duration: exactly 5 minutes from database claim time.
- Lease IDs: cryptographically random UUIDs generated before claim.
- No lease renewal.
- Database claim/reclaim uses `FOR UPDATE SKIP LOCKED` and server timestamps.
- Commit/release/cancel SQL must match job ID, `MEASURING`, lease ID and unexpired lease.
- Recovery replaces the expired lease atomically, increments attempts and records a new immutable attempt fact.
- A measurement row remains unique by job/document version; fencing is mandatory and uniqueness is only defense in depth.

### Failure classification

Terminal without automatic retry:

- `INPUT_HASH_MISMATCH`
- `INPUT_SIZE_MISMATCH`
- `UNSUPPORTED_OR_MISMATCHED_FORMAT`
- `ENCRYPTED_DOCUMENT`
- `CORRUPT_DOCUMENT`
- `PAGE_LIMIT_EXCEEDED`
- `CHARACTER_LIMIT_EXCEEDED`
- `ARCHIVE_LIMIT_EXCEEDED`
- `XML_LIMIT_EXCEEDED`
- `RESOURCE_LIMIT_EXCEEDED`
- `PARSER_TIMEOUT`
- current-authority denial

Transient and eligible for the bounded schedule:

- exact-object storage temporarily unavailable before bytes are consumed;
- parser executable/process start temporarily unavailable (`PARSER_UNAVAILABLE`);
- credit-calculation or ID-generation dependency unavailable before measurement commit;
- worker/process interruption represented by lease expiry;
- clearly rolled-back database operation before commit.

Unknown failures default to terminal, not transient. An ambiguous PostgreSQL commit is reconciled by reading the job/measurement evidence before any reclaim; it is never blindly repeated.

### Attempt evidence and privacy

Each attempt records immutable minimal facts:

- job/document version IDs;
- lease ID, attempt number, claim/expiry/completion times;
- parser/semantics/profile versions when parsing began;
- outcome `SUCCEEDED`, `TRANSIENT_FAILURE`, `TERMINAL_FAILURE` or `LEASE_EXPIRED`;
- allowlisted stable failure code.

Attempt evidence excludes object keys, filenames, document bytes/text, parser stderr, stack traces, credentials and provider data. Private wrapped diagnostics remain process-local.

### State and scheduling

- A transient failure after confirmed process termination returns the job to `QUEUED` with an explicit `next_attempt_at`.
- Claiming requires `next_attempt_at <= database now`.
- Lease expiry recovery runs before new measurement claims but after reconciliation of already committed measurements awaiting entitlement linkage.
- Terminal failure moves the job to `FAILED`, clears lease fields, sets completion time and prevents entitlement reservation/provider execution.
- Archive remains blocked while a lease or retry is active.
- No automatic reset of a terminal job is included; any future human retry requires a separately approved operation and authority.

### Process safety

Fencing protects database effects but does not stop a stale parser from consuming CPU or memory. Runtime composition therefore remains blocked until Linux evidence proves process-group/cgroup termination, no-network execution and the approved memory/CPU/wall ceilings. A worker must not intentionally reclaim early merely because its local context was cancelled.

## Not decided here

- Human/operator reset of terminal measurement jobs.
- Changing parser/profile versions and remeasuring historical documents.
- Fleet-wide scheduler cadence or deployment replica count.
- Alert thresholds and operational escalation.
- Retention/deletion of immutable attempt evidence.

The approved policy is represented in source through Tenderpreneur migration `00030_document_measurement_leases.sql`, fenced application/store operations and immutable attempt evidence. The worker remains unmounted pending parser/Linux isolation, migration and verification gates.
