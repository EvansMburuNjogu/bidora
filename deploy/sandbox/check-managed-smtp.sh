#!/usr/bin/env bash
# Run on the sandbox host before replacing the backend with managed-only SMTP.
# Read-only: no credentials, mail sends, activation or database mutations.
set -euo pipefail
cd "$(dirname "$0")/../.."
active=$(docker compose -f deploy/sandbox/compose.yml exec -T postgres \
  psql -X -v ON_ERROR_STOP=1 -U postgres -d bidora_sandbox_control -At -c \
  "SELECT count(*) FROM platform_settings.integration_heads h
   JOIN platform_settings.integration_versions v
     ON v.id=h.active_version_id AND v.setting_kind='SMTP' AND v.state='ACTIVE'
   WHERE h.setting_kind='SMTP';")
if [[ "$active" != "1" ]]; then
  printf '%s\n' 'BLOCKED: managed SMTP is not active. Preserve the working backend; a draft or pending approval cannot send login codes.' >&2
  exit 1
fi
if grep -Eq '^[[:space:]]*(export[[:space:]]+)?BIDORA_PLATFORM_LOGIN_SMTP_[A-Z_]+[[:space:]]*=' /opt/bidora-sandbox/secrets/runtime.env; then
  printf '%s\n' 'BLOCKED: obsolete Platform login SMTP keys remain. Back up and remove them before starting the managed-only backend.' >&2
  exit 1
fi
printf '%s\n' 'Managed SMTP deployment prerequisites passed. This does not verify provider availability or inbox delivery.'
