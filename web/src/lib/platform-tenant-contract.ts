export const platformTenantStates = ["REQUESTED", "PROVISIONING", "MIGRATING", "SEEDING", "ACTIVE", "FAILED", "SUSPENDED", "DECOMMISSIONED"] as const;
export type PlatformTenantState = (typeof platformTenantStates)[number];
export type PlatformTenantSchema = { current_version: number; generation: number; state: "COMPATIBLE" | "MIGRATING" | "FAILED" | "INCOMPATIBLE"; updated_at: string };
export type PlatformTenant = {
  id: string; name: string; tenant_type: "TENDERPRENEUR"; state: PlatformTenantState;
  country: string; timezone: string; locale: string; assignment_generation: number; version: number;
  assignment_present: boolean; common_schema: PlatformTenantSchema | null; tenant_schema: PlatformTenantSchema | null;
  created_at: string; updated_at: string;
};
export type PlatformTenantList = { tenants: PlatformTenant[]; next_cursor: string | null };
const validUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function keys(v: Record<string, unknown>, required: readonly string[]): boolean { return Object.keys(v).length === required.length && required.every((key) => Object.hasOwn(v, key)); }
function positive(v: unknown): v is number { return Number.isSafeInteger(v) && Number(v) > 0; }
function timestamp(v: unknown): v is string { return typeof v === "string" && v.length <= 40 && /^\d{4}-\d\d-\d\dT/.test(v) && Number.isFinite(Date.parse(v)); }
function schema(v: unknown): PlatformTenantSchema | null | undefined {
  if (v === null) return null;
  const s = record(v);
  if (!s || !keys(s,["current_version","generation","state","updated_at"]) || !positive(s.current_version) || !positive(s.generation) ||
      !["COMPATIBLE","MIGRATING","FAILED","INCOMPATIBLE"].includes(String(s.state)) || !timestamp(s.updated_at)) return undefined;
  return s as PlatformTenantSchema;
}
function tenant(raw: unknown): PlatformTenant | null {
  const t = record(raw);
  if (!t || !keys(t,["id","name","tenant_type","state","country","timezone","locale","assignment_generation","version","assignment_present","common_schema","tenant_schema","created_at","updated_at"]) ||
      typeof t.id !== "string" || !validUUID.test(t.id) || typeof t.name !== "string" || t.name.length < 1 || t.name.length > 300 ||
      t.tenant_type !== "TENDERPRENEUR" || !platformTenantStates.includes(t.state as PlatformTenantState) ||
      typeof t.country !== "string" || !/^[A-Z]{2}$/.test(t.country) || typeof t.timezone !== "string" || !t.timezone || t.timezone.length > 128 ||
      typeof t.locale !== "string" || !t.locale || t.locale.length > 35 || !Number.isSafeInteger(t.assignment_generation) || Number(t.assignment_generation) < 0 ||
      !positive(t.version) || typeof t.assignment_present !== "boolean" || !timestamp(t.created_at) || !timestamp(t.updated_at)) return null;
  const common = schema(t.common_schema), specific = schema(t.tenant_schema);
  if (common === undefined || specific === undefined) return null;
  return { ...t, common_schema: common, tenant_schema: specific } as PlatformTenant;
}
function envelope(value: unknown): { data: unknown; cursor: string | null } | null {
  const root = record(value), meta = record(root?.meta);
  if (!root || !meta || !keys(root,["data","meta"]) || !keys(meta,["trace_id","next_cursor"]) ||
      typeof meta.trace_id !== "string" || meta.trace_id.length < 1 ||
      !(meta.next_cursor === null || typeof meta.next_cursor === "string" && /^[A-Za-z0-9_-]{1,512}$/.test(meta.next_cursor))) return null;
  return { data: root.data, cursor: meta.next_cursor as string | null };
}
export function parsePlatformTenantList(value: unknown): PlatformTenantList | null {
  const result = envelope(value), data = record(result?.data);
  if (!result || !data || !keys(data,["tenants"]) || !Array.isArray(data.tenants) || data.tenants.length > 100) return null;
  const tenants: PlatformTenant[] = [], seen = new Set<string>();
  for (const row of data.tenants) {
    const item = tenant(row);
    if (!item || seen.has(item.id.toLowerCase())) return null;
    seen.add(item.id.toLowerCase()); tenants.push(item);
  }
  return { tenants, next_cursor: result.cursor };
}
export function parsePlatformTenantDetail(value: unknown, id: string): PlatformTenant | null {
  const result = envelope(value);
  if (!result || result.cursor !== null) return null;
  const item = tenant(result.data);
  return item?.id.toLowerCase() === id.toLowerCase() ? item : null;
}
