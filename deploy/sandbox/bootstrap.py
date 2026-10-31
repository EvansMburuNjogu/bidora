#!/usr/bin/env python3
"""Generate fresh sandbox-only deployment secrets. Never prints secret values."""
import base64
import os
from pathlib import Path
import secrets
import subprocess
import urllib.parse

base = Path('/opt/bidora-sandbox/secrets')
if base.exists() and any(base.iterdir()):
    raise SystemExit('Sandbox secrets already exist; refusing to replace them.')
os.umask(0o077)
base.mkdir(parents=True, exist_ok=True)
base.chmod(0o700)
tls = base / 'tls'
tls.mkdir(mode=0o755)
tls.chmod(0o755)
def run(*args):
    subprocess.run(args, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
def write(name, data):
    path = base / name
    path.write_text(data)
    path.chmod(0o600)
run('openssl', 'req', '-x509', '-newkey', 'rsa:3072', '-nodes', '-days', '365', '-subj', '/CN=Bidora Sandbox Internal CA', '-keyout', str(tls/'ca.key'), '-out', str(tls/'ca.crt'))
(tls/'ca.crt').chmod(0o644)
for name in ('postgres', 'redis'):
    ext = tls / (name + '.ext')
    ext.write_text(f'subjectAltName=DNS:{name}\nextendedKeyUsage=serverAuth\n')
    run('openssl', 'req', '-newkey', 'rsa:3072', '-nodes', '-subj', '/CN='+name, '-keyout', str(tls/(name+'.key')), '-out', str(tls/(name+'.csr')))
    run('openssl', 'x509', '-req', '-in', str(tls/(name+'.csr')), '-CA', str(tls/'ca.crt'), '-CAkey', str(tls/'ca.key'), '-CAcreateserial', '-days', '365', '-extfile', str(ext), '-out', str(tls/(name+'.crt')))
    (tls/(name+'.crt')).chmod(0o644)
admin, runtime, redis = (secrets.token_hex(32) for _ in range(3))
write('postgres.env', f'POSTGRES_DB=bidora_sandbox_control\nPOSTGRES_USER=postgres\nPOSTGRES_PASSWORD={admin}\nPOSTGRES_INITDB_ARGS=--auth-host=scram-sha-256\n')
write('init-runtime.sql', f"CREATE SCHEMA bidora_migrations;\nCREATE ROLE bidora_sandbox_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '{runtime}';\nREVOKE ALL ON DATABASE bidora_sandbox_control FROM PUBLIC;\nGRANT CONNECT ON DATABASE bidora_sandbox_control TO bidora_sandbox_runtime;\nREVOKE CREATE ON SCHEMA public FROM PUBLIC;\n")
write('require-tls.sh', '#!/bin/sh\nset -eu\n{ printf "hostnossl all all all reject\\n"; cat "$PGDATA/pg_hba.conf"; } > "$PGDATA/pg_hba.conf.sandbox"\nmv "$PGDATA/pg_hba.conf.sandbox" "$PGDATA/pg_hba.conf"\n')
write('redis.conf', f'bind 0.0.0.0\nprotected-mode yes\nport 0\ntls-port 6379\ntls-cert-file /run/tls/redis.crt\ntls-key-file /run/tls/redis.key\ntls-ca-cert-file /run/tls/ca.crt\ntls-auth-clients no\nappendonly yes\nmaxmemory 128mb\nmaxmemory-policy noeviction\nuser default off\nuser bidora on >{redis} ~bidora:sandbox:* +@all -@dangerous\n')
def dsn(user, password, migrations=False):
    q={'sslmode':'verify-full','sslrootcert':'/run/bidora/ca.crt'}
    if migrations: q['options']='-c search_path=bidora_migrations,pg_catalog'
    return f'postgresql://{user}:{password}@postgres:5432/bidora_sandbox_control?'+urllib.parse.urlencode(q, quote_via=urllib.parse.quote)
values={
 'BIDORA_ENV':'staging', 'BIDORA_HTTP_ADDR':'0.0.0.0:8080',
 'BIDORA_ALLOWED_ORIGINS':'https://sandbox.bidora.africa',
 'BIDORA_LOCAL_TRUSTED_PROXY_CIDRS':'172.30.42.0/24',
 'BIDORA_TEST_CONTROL_RUNTIME_DSN':dsn('bidora_sandbox_runtime',runtime),
 'BIDORA_TEST_POSTGRES_CA_FILE':'/run/bidora/ca.crt',
 'BIDORA_TEST_REDIS_ADDR':'redis:6379', 'BIDORA_TEST_REDIS_USERNAME':'bidora',
 'BIDORA_TEST_REDIS_PASSWORD':redis, 'BIDORA_TEST_REDIS_CA_FILE':'/run/bidora/ca.crt',
 'BIDORA_REDIS_TLS_SERVER_NAME':'redis', 'BIDORA_LOCAL_SMTP_VAULT_KEY_VERSION':'1',
 'BIDORA_LOCAL_NOTIFICATION_FINGERPRINT_KEY_VERSION':'1', 'BIDORA_PESAPAL_ALLOW_LIVE':'false',
}
for key in ('ATTEMPT_SECRET','EMAIL_LOGIN_HMAC_SECRET','MFA_KEY','SMTP_VAULT_KEY','NOTIFICATION_FINGERPRINT_KEY'):
    values['BIDORA_LOCAL_'+key+'_B64']=base64.b64encode(secrets.token_bytes(32)).decode()
write('runtime.env', ''.join(f'{k}={v}\n' for k,v in values.items()))
write('migrator.env', f'BIDORA_MIGRATION_DSN={dsn("postgres",admin,True)}\nBIDORA_GRANT_DSN={dsn("postgres",admin)}\nBIDORA_RUNTIME_DSN={dsn("bidora_sandbox_runtime",runtime)}\n')
# Match the official image service UIDs without making secret files public.
for owner, names in [('70:70', ['tls/postgres.key', 'init-runtime.sql', 'require-tls.sh']), ('999:999', ['tls/redis.key', 'redis.conf'])]:
    subprocess.run(['sudo', '-n', 'chown', owner, *[str(base/name) for name in names]], check=True)
print('Fresh sandbox secrets generated; provider activation remains disabled.')
