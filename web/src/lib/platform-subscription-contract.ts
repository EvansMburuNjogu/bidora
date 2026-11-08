export type PlatformSubscription = { tenant_id: string; subscription_id: string; state: string; access_mode: string; version: number; plan_key: string; catalogue_version: string; cadence: string; commercial_period_end: string; cancel_at: string | null };
export type PlatformSubscriptionInventoryRow = { tenant_name?: string; tenant_id: string; subscription_id: string; state: string; version: number; plan_key: string; cadence: string; has_current_term: boolean; commercial_period_end: string | null; cancel_at: string | null; updated_at: string };
export type PlatformSubscriptionInventoryPage = { subscriptions: PlatformSubscriptionInventoryRow[]; next_cursor: string | null };
export const platformSubscriptionStates = ["TRIAL_ACTIVE", "PAYMENT_PENDING", "ACTIVE", "PAST_DUE", "SUSPENDED", "ENDED", "REVIEW_REQUIRED"] as const;
const uuid = /^[a-f\d]{8}-[a-f\d]{4}-[1-8][a-f\d]{3}-[89ab][a-f\d]{3}-[a-f\d]{12}$/i;
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function utc(v: unknown): v is string { return typeof v === "string" && !Number.isNaN(Date.parse(v)) && (v.endsWith("Z") || v.endsWith("+00:00")); }
export function parsePlatformSubscriptionInventory(v: unknown): PlatformSubscriptionInventoryPage | null {
 const root=record(v),data=record(root?.data),meta=record(root?.meta);
 if(!root||!data||!meta||typeof meta.trace_id!=="string"||!meta.trace_id||!Array.isArray(data.subscriptions)||data.subscriptions.length>50||!(meta.next_cursor===null||typeof meta.next_cursor==="string"&&uuid.test(meta.next_cursor)))return null;
 const rows:PlatformSubscriptionInventoryRow[]=[];
 for(const entry of data.subscriptions){const row=record(entry);
  if(!row||row.tenant_name!==undefined&&(typeof row.tenant_name!=="string"||row.tenant_name.length>200)||!uuid.test(String(row.tenant_id))||!uuid.test(String(row.subscription_id))||!platformSubscriptionStates.includes(row.state as typeof platformSubscriptionStates[number])||!Number.isSafeInteger(row.version)||Number(row.version)<1||typeof row.plan_key!=="string"||!(["","TRIAL","BASIC","GROWTH","PROFESSIONAL","ENTERPRISE"] as string[]).includes(row.plan_key)||typeof row.cadence!=="string"||!(["","TRIAL","MONTHLY","ANNUAL","CONTRACT"] as string[]).includes(row.cadence)||typeof row.has_current_term!=="boolean"||!(row.commercial_period_end===null||utc(row.commercial_period_end))||row.has_current_term!==(row.commercial_period_end!==null)||row.has_current_term&&(row.plan_key===""||row.cadence==="")||!(row.cancel_at===null||utc(row.cancel_at))||!utc(row.updated_at)||rows.some((previous)=>previous.subscription_id.toLowerCase()>=String(row.subscription_id).toLowerCase()))return null;
  rows.push(row as PlatformSubscriptionInventoryRow);
 }
 if(meta.next_cursor!==null&&(!rows.length||meta.next_cursor.toLowerCase()!==rows.at(-1)?.subscription_id.toLowerCase()))return null;
 return {subscriptions:rows,next_cursor:meta.next_cursor as string|null};
}
export function parsePlatformSubscription(v: unknown, requestedTenant: string): PlatformSubscription | null {
  const root = record(v), data = record(root?.data), sub = record(data?.subscription), meta = record(root?.meta);
  if (!root || !data || !sub || !meta || typeof meta.trace_id !== "string" || data.tenant_id !== requestedTenant || !uuid.test(requestedTenant) || !uuid.test(String(sub.subscription_id)) || !Number.isSafeInteger(sub.version) || Number(sub.version) < 1 || !["TRIAL_ACTIVE", "ACTIVE", "PAST_DUE", "SUSPENDED", "ENDED", "REVIEW_REQUIRED", "PAYMENT_PENDING"].includes(String(sub.state)) || !["FULL", "RETAINED_READ_ONLY", "LAST_RECONCILED"].includes(String(sub.access_mode)) || typeof sub.plan_key !== "string" || typeof sub.catalogue_version !== "string" || typeof sub.cadence !== "string" || !utc(sub.commercial_period_end) || (sub.cancel_at !== null && !utc(sub.cancel_at))) return null;
  return { tenant_id: requestedTenant, subscription_id: sub.subscription_id as string, state: sub.state as string, access_mode: sub.access_mode as string, version: sub.version as number, plan_key: sub.plan_key, catalogue_version: sub.catalogue_version, cadence: sub.cadence, commercial_period_end: sub.commercial_period_end, cancel_at: sub.cancel_at as string | null };
}
export function parsePlatformSubscriptionReceipt(v: unknown, tenant: string): { version: number; cancel_at: string | null; replay: boolean } | null {
  const root = record(v), data = record(root?.data), meta = record(root?.meta);
  if (!data || !meta || typeof meta.trace_id !== "string" || data.tenant_id !== tenant || !uuid.test(String(data.subscription_id)) || !Number.isSafeInteger(data.version) || Number(data.version) < 2 || typeof data.replay !== "boolean" || data.payment_executed !== false || (data.cancel_at !== null && !utc(data.cancel_at))) return null;
  return { version: data.version as number, cancel_at: data.cancel_at as string | null, replay: data.replay };
}
