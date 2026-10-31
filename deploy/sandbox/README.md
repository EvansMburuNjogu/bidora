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
docker compose -f deploy/sandbox/compose.yml build backend web
# Apply only reviewed forward migrations for this designated sandbox database.
docker compose -f deploy/sandbox/compose.yml run --rm migrate
docker compose -f deploy/sandbox/compose.yml run --rm grants
docker compose -f deploy/sandbox/compose.yml up -d backend web
docker compose -f deploy/sandbox/compose.yml ps
```

Do not run `down -v`, reset databases, prune unrelated Docker resources or reuse production credentials. Initial `bootstrap.py` refuses to replace existing secrets. PostgreSQL initialization creates the migration ledger schema and denies runtime database CREATE/TEMPORARY privileges; runtime table grants come from the existing grant command.

Nginx configuration is sandbox-only. The web BFF reaches the API through verified HTTPS using a host-gateway mapping, avoiding a Cloudflare round trip without weakening TLS. Only the isolated sandbox network is trusted to preserve BFF-forwarded client addresses; public caller headers are overwritten. Cloudflare ranges apply only to the sandbox virtual hosts.

The dedicated Let's Encrypt certificate renews through the existing Certbot timer; `renew-hook.sh` reloads Nginx only for this certificate. This deployment runs the existing identity/Platform API, not the full tenant document-processing worker stack.

No initial administrator, tenant data or provider credentials are copied from local or production environments. Initial Owner setup and provider configuration remain separate. SMTP/AI trust bundles, when needed, must be supplied to the frontend server through the deployment-owned `NODE_EXTRA_CA_CERTS` or `SSL_CERT_FILE` configuration. Paid/live provider operations remain disabled.
