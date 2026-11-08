export type OpenAISetting = { pending_change_id?: string | null; id: string; setting_kind: "OPENAI"; sequence: number; state: string; version: number; test_status: string; test_code: string | null; created_at: string; tested_at: string | null; activated_at: string | null; superseded_at: string | null; configuration: { enabled?: boolean; setting_kind: "OPENAI"; endpoint: "https://api.openai.com/v1/responses"; model: string; api_key_reference_configured: true; trust_roots_reference_configured: true; store: false; timeout_seconds: number; max_concurrency: number } };
export type OpenAISettingPage = { versions: OpenAISetting[]; next_cursor: string | null };
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function exact(v: Record<string, unknown>, keys: readonly string[]): boolean { return Object.keys(v).length === keys.length && keys.every((key) => Object.hasOwn(v, key)); }
function timestamp(v: unknown): boolean { return typeof v === "string" && v.length <= 40 && /^\d{4}-\d\d-\d\dT.*(?:Z|[+-]\d\d:\d\d)$/.test(v) && Number.isFinite(Date.parse(v)); }
function version(raw: unknown): OpenAISetting | null {
  const source = record(raw); if (!source) return null;
  const { pending_change_id, ...v } = source;
  if (pending_change_id !== undefined && pending_change_id !== null && (typeof pending_change_id !== "string" || !uuid.test(pending_change_id))) return null;
  const c = record(v.configuration);
  if (!v || !c || !exact(v, ["id", "setting_kind", "sequence", "state", "version", "configuration", "test_status", "test_code", "created_at", "tested_at", "activated_at", "superseded_at"]) ||
    !(exact(c, ["setting_kind", "endpoint", "model", "api_key_reference_configured", "trust_roots_reference_configured", "store", "timeout_seconds", "max_concurrency"]) || exact(c, ["enabled", "setting_kind", "endpoint", "model", "api_key_reference_configured", "trust_roots_reference_configured", "store", "timeout_seconds", "max_concurrency"]) && typeof c.enabled === "boolean") ||
    typeof v.id !== "string" || !uuid.test(v.id) || v.setting_kind !== "OPENAI" || !Number.isSafeInteger(v.sequence) || Number(v.sequence) < 1 || !Number.isSafeInteger(v.version) || Number(v.version) < 1 ||
    !["DRAFT", "TEST_PASSED", "TEST_FAILED", "PENDING_APPROVAL", "ACTIVE", "SUPERSEDED", "REJECTED"].includes(String(v.state)) ||
    !["NOT_TESTED", "PASSED", "FAILED"].includes(String(v.test_status)) || !(v.test_code === null || typeof v.test_code === "string" && v.test_code.length <= 80) ||
    !timestamp(v.created_at) || !["tested_at", "activated_at", "superseded_at"].every((name) => v[name] === null || timestamp(v[name])) ||
    c.setting_kind !== "OPENAI" || c.endpoint !== "https://api.openai.com/v1/responses" || typeof c.model !== "string" || c.model.length < 1 || c.model.length > 200 ||
    c.api_key_reference_configured !== true || c.trust_roots_reference_configured !== true || c.store !== false || !Number.isSafeInteger(c.timeout_seconds) || Number(c.timeout_seconds) < 10 || Number(c.timeout_seconds) > 900 || !Number.isSafeInteger(c.max_concurrency) || Number(c.max_concurrency) < 1 || Number(c.max_concurrency) > 100) return null;
  return { ...v, ...(pending_change_id !== undefined ? {pending_change_id} : {}) } as OpenAISetting;
}
function envelope(v: unknown): { data: unknown; cursor: string | null } | null {
  const r = record(v), m = record(r?.meta);
  if (!r || !m || !exact(r, ["data", "meta"]) || !(exact(m, ["trace_id"]) || exact(m, ["trace_id", "next_cursor"])) || typeof m.trace_id !== "string" || !m.trace_id ||
      !(m.next_cursor === undefined || m.next_cursor === null || typeof m.next_cursor === "string" && /^[A-Za-z0-9_-]{1,256}$/.test(m.next_cursor))) return null;
  return { data: r.data, cursor: m.next_cursor === undefined ? null : m.next_cursor as string | null };
}
export function parseOpenAISetting(raw: unknown): OpenAISetting | null { const e = envelope(raw); return e && e.cursor === null ? version(e.data) : null; }
export function parseOpenAISettingPage(raw: unknown): OpenAISettingPage | null {
  const e = envelope(raw);
  if (!e || !Array.isArray(e.data) || e.data.length > 100) return null;
  const seen = new Set<string>(), versions: OpenAISetting[] = [];
  for (const rawVersion of e.data) { const parsed = version(rawVersion); if (!parsed || seen.has(parsed.id.toLowerCase())) return null; versions.push(parsed); seen.add(parsed.id.toLowerCase()); }
  return { versions, next_cursor: e.cursor };
}
