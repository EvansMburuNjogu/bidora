# Bidora sandbox

- Web: https://sandbox.bidora.africa
- API readiness: https://api-sandbox.bidora.africa/health/ready
- SSH alias: `maalim-server` (not `maalim-servver`).
- Checkout: `/opt/bidora-sandbox/app`, branch `sandbox`.
- Secrets: `/opt/bidora-sandbox/secrets`, outside Git; never print or commit these files.
- Compose project: `bidora-sandbox`; isolated control database `bidora_sandbox_control` and Redis. PostgreSQL/Redis use verified internal TLS; application containers run non-root. Only Nginx-facing loopback ports 3100 and 18080 are published.

Update from GitHub:

```sh
cd /opt/bidora-sandbox/app
git pull --ff-only origin sandbox
# Mandatory before replacing the working backend with managed-only SMTP:
# stop here if the saved SMTP revision is still a draft/pending approval.
bash deploy/sandbox/check-managed-smtp.sh
docker compose -f deploy/sandbox/compose.yml build backend web
# Apply only reviewed forward migrations for this designated sandbox database.
docker compose -f deploy/sandbox/compose.yml run --rm migrate
docker compose -f deploy/sandbox/compose.yml run --rm grants
docker compose -f deploy/sandbox/compose.yml up -d backend web
docker compose -f deploy/sandbox/compose.yml ps
```

## Current audit rollout and rollback

Control schema/ledger are now **50**. The audit web source is `26471c0`; the running backend is the login-compatible `6ca77a8` from `sandbox-email-recovery`, not the managed-only main runtime. The existing login configuration was preserved; unrelated business handlers and provider activations were not deployed. Private database backups and pre-upgrade images are retained outside Git.

For binary rollback at schema 50, use the retained `bidora-sandbox/backend:audit-schema50-rollback` image (source `39fb8dc`, branch `sandbox-audit-rollback`), first running **that image's** `grant-runtime -plane control -manifest-version 1` with the existing designated migrator environment/TLS mount, then retag/recreate only the backend. Its grant manifest restores the earlier audit projection access without granting raw audit reads. Retain the previous web image `bidora-sandbox/web:before-audit-schema50` if frontend rollback is also necessary. The original schema-43 backend cannot simply be restarted against schema 50. Never run destructive down migrations or restore over the live database; database recovery remains restore-to-new under explicit authorization.

Readiness is not authentication readiness: `/health/ready` can pass while managed SMTP is pending and login-code delivery fails. `check-managed-smtp.sh` is a read-only deployment gate; it neither approves pending changes nor sends email. Keep the previous backend image and private runtime environment backup until an authorized login verifies the new delivery path. During the reported login-503 recovery, the previous backend image and its existing deployment login-channel configuration were restored; this is a temporary rollback, not delivery from the unactivated managed revision. Do not replace that working image with managed-only SMTP until the gate passes.

Do not run `down -v`, reset databases, prune unrelated Docker resources or reuse production credentials. Initial `bootstrap.py` refuses to replace existing secrets. PostgreSQL initialization creates the migration ledger schema and denies runtime database CREATE/TEMPORARY privileges; runtime table grants come from the existing grant command.

Nginx configuration is sandbox-only. The web BFF reaches the API through verified HTTPS using a host-gateway mapping, avoiding a Cloudflare round trip without weakening TLS. Only the isolated sandbox network is trusted to preserve BFF-forwarded client addresses; public caller headers are overwritten. Cloudflare ranges apply only to the sandbox virtual hosts.

The dedicated Let's Encrypt certificate renews through the existing Certbot timer; `renew-hook.sh` reloads Nginx only for this certificate. This deployment runs the existing identity/Platform API, not the full tenant document-processing worker stack.

The web SMTP draft proxy requires a deployment-owned PEM trust root before it may submit a draft without browser-provided certificate material. Provision `/opt/bidora-sandbox/secrets/tls/smtp-roots.pem` as a readable, bounded public CA certificate bundle, verified against the configured SMTP host (Zoho currently chains via Thawte TLS RSA CA G1 to DigiCert Global Root G2). Compose mounts it read-only as `NODE_EXTRA_CA_CERTS`; do not put private keys or SMTP credentials in this file, bypass TLS checks or use a large generic system bundle. Recheck the host chain before replacing a root. Missing/unreadable trust fails closed with a 503; a draft/test is not provider activation or inbox-delivery proof.

No initial administrator, tenant data or provider credentials are copied from local or production environments. Initial Owner setup and provider configuration remain separate.

## Initial Owner setup

`/usr/local/bin/sandbox-owner-bootstrap` supports only `verify-email PRIVATE_INPUT_PATH` and `promote PRIVATE_INPUT_PATH`. Run it explicitly in an ephemeral backend-image container with the sandbox network, staging runtime environment, verified PostgreSQL CA and a mode-0600 private input bind mount. Do not add it to application startup or a worker loop. The existing identity must first be registered normally; the input selects that identity by email, never by password hash or caller-selected DSN.

`verify-email` is a one-off, authenticated STARTTLS Zoho sender for the unconsumed sandbox bootstrap only. It issues the normal expiring challenge and sends the token only to the identity's mailbox. It does not activate a managed SMTP revision or install a server fallback. Provider acceptance is not inbox-delivery evidence; keep its correlation/status output in the private deployment directory, and do not blindly resend after ambiguous failure.

The operator must verify the mailbox at `/verify-email`, sign in, then enroll an authenticator at `/account/security/authenticator` before promotion. Do not select a tenant account type or create an organization for this identity. `promote` delegates to the existing one-shot bootstrap transaction: it requires verified email, active MFA, an unclassified identity, no tenant ties and no prior Owner history; it creates the Owner role with audit/outbox evidence and revokes prior sessions. It cannot assign arbitrary additional roles or create a second Owner. After successful promotion, retire the private bootstrap input; managed SMTP setup retains normal permissions, encrypted storage and distinct-owner approval. SMTP/AI trust bundles, when needed, must be supplied to the frontend server through the deployment-owned `NODE_EXTRA_CA_CERTS` or `SSL_CERT_FILE` configuration. Paid/live provider operations remain disabled.
