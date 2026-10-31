export const smtpUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
export type SMTPVersion = { id: string; setting_kind: "SMTP"; sequence: number; version: number; state: "DRAFT" | "TEST_PASSED" | "TEST_FAILED" | "PENDING_APPROVAL" | "ACTIVE" | "SUPERSEDED" | "REJECTED";
  test_status: "NOT_TESTED" | "PASSED" | "FAILED"; test_code: string | null; created_at: string; tested_at: string | null; activated_at: string | null; superseded_at: string | null;
  configuration: { setting_kind: "SMTP"; host: string; port: number; credential_reference_configured: true; trust_roots_reference_configured: true; starttls_required: true; sender_name: string; sender_address: string; reply_to_address: string | null; connect_timeout_seconds: number; command_timeout_seconds: number; delivery_timeout_seconds: number; max_concurrency: number } };
export type SMTPChange = { id: string; setting_kind: "SMTP"; target_version_id: string; action: "ACTIVATE" | "ROLLBACK"; state: "PENDING_APPROVAL" | "APPROVED_EXECUTED" | "REJECTED" | "EXPIRED";
  version: number; maker_user_id: string; checker_user_id: string | null; created_at: string; expires_at: string; decided_at: string | null };
export type SMTPVersionPage = { versions: SMTPVersion[]; next_cursor: string | null };
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function exact(v: Record<string, unknown>, keys: readonly string[]): boolean { return Object.keys(v).length === keys.length && keys.every((key) => Object.hasOwn(v, key)); }
const id = (v: unknown): v is string => typeof v === "string" && smtpUUID.test(v);
const positive = (v: unknown): v is number => Number.isSafeInteger(v) && Number(v) > 0;
const bounded = (v: unknown, max = 512): v is string => typeof v === "string" && v.length > 0 && v.length <= max;
const instant = (v: unknown): v is string => typeof v === "string" && v.length <= 40 && /^\d{4}-\d\d-\d\dT.*(?:Z|[+-]\d\d:\d\d)$/.test(v) && Number.isFinite(Date.parse(v));
const maybeInstant = (v: unknown): boolean => v === null || instant(v);
function envelope(v: unknown): { data: unknown; cursor: string | null } | null {
  const root = record(v), meta = record(root?.meta);
  if (!root || !meta || !exact(root, ["data", "meta"]) || !bounded(meta.trace_id) ||
      !(meta.next_cursor === undefined || meta.next_cursor === null || typeof meta.next_cursor === "string" && /^[A-Za-z0-9_-]{1,256}$/.test(meta.next_cursor))) return null;
  // The version-list handler supplies next_cursor; single-record responses may omit it.
  if (!(exact(meta, ["trace_id", "next_cursor"]) || exact(meta, ["trace_id"]))) return null;
  return { data: root.data, cursor: meta.next_cursor === undefined ? null : meta.next_cursor as string | null };
}
function version(raw: unknown): SMTPVersion | null {
  const v = record(raw), c = record(v?.configuration);
  if (!v || !c || !exact(v, ["id", "setting_kind", "sequence", "state", "version", "configuration", "test_status", "test_code", "created_at", "tested_at", "activated_at", "superseded_at"]) ||
      !exact(c, ["setting_kind", "host", "port", "credential_reference_configured", "trust_roots_reference_configured", "starttls_required", "sender_name", "sender_address", "reply_to_address", "connect_timeout_seconds", "command_timeout_seconds", "delivery_timeout_seconds", "max_concurrency"]) ||
      !id(v.id) || v.setting_kind !== "SMTP" || !positive(v.sequence) || !positive(v.version) || !["DRAFT", "TEST_PASSED", "TEST_FAILED", "PENDING_APPROVAL", "ACTIVE", "SUPERSEDED", "REJECTED"].includes(String(v.state)) ||
      !["NOT_TESTED", "PASSED", "FAILED"].includes(String(v.test_status)) || !(v.test_code === null || bounded(v.test_code, 80)) || !instant(v.created_at) || !maybeInstant(v.tested_at) || !maybeInstant(v.activated_at) || !maybeInstant(v.superseded_at) ||
      c.setting_kind !== "SMTP" || !bounded(c.host, 253) || !positive(c.port) || Number(c.port) > 65535 || c.credential_reference_configured !== true || c.trust_roots_reference_configured !== true || c.starttls_required !== true || !bounded(c.sender_name, 200) || !bounded(c.sender_address, 254) || !(c.reply_to_address === null || bounded(c.reply_to_address, 254)) ||
      !["connect_timeout_seconds", "command_timeout_seconds", "delivery_timeout_seconds", "max_concurrency"].every((key) => positive(c[key]))) return null;
  return v as SMTPVersion;
}
function change<K extends "SMTP" | "OPENAI" | "PESAPAL">(raw: unknown, kind: K): (Omit<SMTPChange,"setting_kind"> & { setting_kind: K }) | null {
  const c = record(raw);
  if (!c || !exact(c, ["id", "setting_kind", "target_version_id", "action", "state", "version", "maker_user_id", "checker_user_id", "created_at", "expires_at", "decided_at"]) ||
      !id(c.id) || c.setting_kind !== kind || !id(c.target_version_id) || !id(c.maker_user_id) || !(c.checker_user_id === null || id(c.checker_user_id)) || !positive(c.version) || !["ACTIVATE", "ROLLBACK"].includes(String(c.action)) || !["PENDING_APPROVAL", "APPROVED_EXECUTED", "REJECTED", "EXPIRED"].includes(String(c.state)) || !instant(c.created_at) || !instant(c.expires_at) || !maybeInstant(c.decided_at)) return null;
  return c as Omit<SMTPChange,"setting_kind"> & { setting_kind: K };
}
export function parseSMTPVersion(v: unknown): SMTPVersion | null { const e = envelope(v); return e && e.cursor === null ? version(e.data) : null; }
export function parseSMTPVersions(v: unknown): SMTPVersionPage | null {
  const e = envelope(v);
  if (!e || !Array.isArray(e.data) || e.data.length > 100) return null;
  const versions: SMTPVersion[] = [], seen = new Set<string>();
  for (const raw of e.data) { const row = version(raw); if (!row || seen.has(row.id.toLowerCase())) return null; versions.push(row); seen.add(row.id.toLowerCase()); }
  return { versions, next_cursor: e.cursor };
}
export function parseIntegrationChange<K extends "SMTP" | "OPENAI" | "PESAPAL">(v: unknown, kind: K, requested?: string): (Omit<SMTPChange,"setting_kind"> & {setting_kind:K}) | null {const e=envelope(v);if(!e||e.cursor!==null)return null;const row=change(e.data,kind);return row&&(!requested||row.id.toLowerCase()===requested.toLowerCase())?row:null;}
export function parseSMTPChange(v: unknown, requested?: string): SMTPChange | null {
  const e = envelope(v);
  if (!e || e.cursor !== null) return null;
  const row = change(e.data,"SMTP");
  return row && (!requested || row.id.toLowerCase() === requested.toLowerCase()) ? row : null;
}
export function parseSMTPDecision(v: unknown, requested: string): SMTPChange | null {
  const e = envelope(v), data = record(e?.data);
  if (!e || e.cursor !== null || !data || !exact(data, ["change", "active_version"]) || !version(data.active_version)) return null;
  const row = change(data.change,"SMTP");
  return row?.id.toLowerCase() === requested.toLowerCase() && row.state === "APPROVED_EXECUTED" ? row : null;
}
