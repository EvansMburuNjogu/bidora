export type ManagedPlanKey = "BASIC" | "GROWTH" | "PROFESSIONAL";
export type ManagedMoney = { currency: "KES"; amount: string; amount_minor_units: number };
export type ManagedPlan = { version: number; head_version: number; monthly_price: ManagedMoney; annual_price: ManagedMoney; additional_licence_monthly_price: ManagedMoney; bid_workspaces: number; included_licences: number; storage_bytes: number; document_versions: number; ai_credits: number; concurrent_ai_jobs: number };
export type ManagedPlanItem = { plan_key: ManagedPlanKey; published: ManagedPlan; draft: ManagedPlan | null; active_version: number; head_version: number };
const keys: ManagedPlanKey[] = ["BASIC", "GROWTH", "PROFESSIONAL"];
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function integer(v: unknown): v is number { return Number.isSafeInteger(v) && Number(v) > 0; }
function money(v: unknown): ManagedMoney | null { const m = record(v); if (!m || m.currency !== "KES" || typeof m.amount !== "string" || !/^(0|[1-9]\d*)\.\d{2}$/.test(m.amount) || !integer(m.amount_minor_units) || (BigInt(m.amount.replace(".", "")) !== BigInt(Number(m.amount_minor_units)))) return null; return m as ManagedMoney; }
function plan(v: unknown): ManagedPlan | null { const p = record(v); if (!p || !integer(p.version) || !Number.isSafeInteger(p.head_version) || Number(p.head_version) < 0 || !integer(p.bid_workspaces) || !integer(p.included_licences) || !integer(p.storage_bytes) || !integer(p.document_versions) || !integer(p.ai_credits) || !integer(p.concurrent_ai_jobs)) return null;
  for (const field of ["monthly_price", "annual_price", "additional_licence_monthly_price"] as const) if (!money(p[field])) return null;
  return p as ManagedPlan;
}
export function parseManagedPlans(v: unknown): ManagedPlanItem[] | null {
  const root = record(v), data = record(root?.data), meta = record(root?.meta);
  if (!root || !data || !meta || typeof meta.trace_id !== "string" || !meta.trace_id || !Array.isArray(data.plans) || data.plans.length !== 3) return null;
  const items: ManagedPlanItem[] = [];
  for (const raw of data.plans) { const item = record(raw); if (!item || !keys.includes(item.plan_key as ManagedPlanKey) || items.some((p) => p.plan_key === item.plan_key) || !integer(item.head_version) || !integer(item.active_version)) return null;
    const published = plan(item.published), draft = item.draft === null ? null : plan(item.draft);
    if (!published || published.version !== item.active_version || published.head_version !== item.head_version || (item.draft !== null && (!draft || draft.version <= published.version))) return null;
    items.push({ plan_key: item.plan_key as ManagedPlanKey, published, draft, active_version: item.active_version as number, head_version: item.head_version as number });
  }
  return items;
}
export function parsePlanReceipt(v: unknown): { plan_key: ManagedPlanKey; version: number; head_version: number; published: boolean } | null {
  const root = record(v), data = record(root?.data), meta = record(root?.meta);
  if (!data || !meta || typeof meta.trace_id !== "string" || !keys.includes(data.plan_key as ManagedPlanKey) || !integer(data.version) || !integer(data.head_version) || typeof data.published !== "boolean" || typeof data.replay !== "boolean") return null;
  return { plan_key: data.plan_key as ManagedPlanKey, version: data.version as number, head_version: data.head_version as number, published: data.published };
}
