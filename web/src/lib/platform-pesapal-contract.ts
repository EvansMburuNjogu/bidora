export type PesaPalSetting={id:string;setting_kind:"PESAPAL";sequence:number;state:string;version:number;test_status:string;test_code:string|null;created_at:string;tested_at:string|null;activated_at:string|null;superseded_at:string|null;configuration:{setting_kind:"PESAPAL";enabled:boolean;environment:"SANDBOX"|"LIVE";merchant_name:string;credential_reference_configured:boolean;notification_id:string|null;api_endpoint:string}};
export type PesaPalSettingPage = { versions: PesaPalSetting[]; next_cursor: string | null };
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function exact(v: Record<string, unknown>, keys: readonly string[]): boolean { return Object.keys(v).length === keys.length && keys.every((key) => Object.hasOwn(v, key)); }
function timestamp(v: unknown): boolean { return typeof v === "string" && v.length <= 40 && /^\d{4}-\d\d-\d\dT.*(?:Z|[+-]\d\d:\d\d)$/.test(v) && Number.isFinite(Date.parse(v)); }
function version(raw: unknown): PesaPalSetting | null {
  const v = record(raw), c = record(v?.configuration);
  if (!v || !c || !exact(v, ["id", "setting_kind", "sequence", "state", "version", "configuration", "test_status", "test_code", "created_at", "tested_at", "activated_at", "superseded_at"]) ||
    !exact(c,["setting_kind","enabled","environment","merchant_name","credential_reference_configured","notification_id","api_endpoint"]) ||
    typeof v.id !== "string" || !uuid.test(v.id) || v.setting_kind !== "PESAPAL" || !Number.isSafeInteger(v.sequence) || Number(v.sequence) < 1 || !Number.isSafeInteger(v.version) || Number(v.version) < 1 ||
    !["DRAFT", "TEST_PASSED", "TEST_FAILED", "PENDING_APPROVAL", "ACTIVE", "SUPERSEDED", "REJECTED"].includes(String(v.state)) ||
    !["NOT_TESTED", "PASSED", "FAILED"].includes(String(v.test_status)) || !(v.test_code === null || typeof v.test_code === "string" && v.test_code.length <= 80) ||
    !timestamp(v.created_at) || !["tested_at", "activated_at", "superseded_at"].every((name) => v[name] === null || timestamp(v[name])) ||
    c.setting_kind !== "PESAPAL" || typeof c.enabled!=="boolean" || !["SANDBOX","LIVE"].includes(String(c.environment)) || typeof c.merchant_name!=="string" || !c.merchant_name.trim() || new TextEncoder().encode(c.merchant_name).length>120 || typeof c.credential_reference_configured!=="boolean" || !(c.notification_id===null||typeof c.notification_id==="string"&&uuid.test(c.notification_id)) || c.enabled&&(c.notification_id===null||!c.credential_reference_configured) || c.api_endpoint!==(c.environment==="SANDBOX"?"https://cybqa.pesapal.com/pesapalv3/api":"https://pay.pesapal.com/v3/api")) return null;
  return v as PesaPalSetting;
}
function envelope(v: unknown): { data: unknown; cursor: string | null } | null {
  const r = record(v), m = record(r?.meta);
  if (!r || !m || !exact(r, ["data", "meta"]) || !(exact(m, ["trace_id"]) || exact(m, ["trace_id", "next_cursor"])) || typeof m.trace_id !== "string" || !m.trace_id ||
      !(m.next_cursor === undefined || m.next_cursor === null || typeof m.next_cursor === "string" && /^[A-Za-z0-9_-]{1,256}$/.test(m.next_cursor))) return null;
  return { data: r.data, cursor: m.next_cursor === undefined ? null : m.next_cursor as string | null };
}
export function parsePesaPalSetting(raw: unknown): PesaPalSetting | null { const e = envelope(raw); return e && e.cursor === null ? version(e.data) : null; }
export function parsePesaPalSettingPage(raw: unknown): PesaPalSettingPage | null {
  const e = envelope(raw);
  if (!e || !Array.isArray(e.data) || e.data.length > 100) return null;
  const seen = new Set<string>(), versions: PesaPalSetting[] = [];
  for (const rawVersion of e.data) { const parsed = version(rawVersion); if (!parsed || seen.has(parsed.id.toLowerCase())) return null; versions.push(parsed); seen.add(parsed.id.toLowerCase()); }
  return { versions, next_cursor: e.cursor };
}
