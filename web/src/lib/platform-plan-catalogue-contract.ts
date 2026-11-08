const keys = ["TRIAL", "BASIC", "GROWTH", "PROFESSIONAL", "ENTERPRISE"] as const;
export type PlanKey = typeof keys[number];
export type PlanMoney = { currency: "KES"; amount: string; kind: "FIXED" | "FROM" };
export type PlanCatalogueEntry = {
  key: PlanKey; name: string; catalogue_version: string; available_cadences: string[]; trial_days: number | null;
  bid_workspace_allowance: { count: number | null; period: string };
  included_licences: number | null;
  limits: { storage_bytes: number | null; completed_document_versions: number | null; ai_credits: number | null; concurrent_ai_jobs: number | null; reset_period: string; rollover: false };
  trial_base_price: PlanMoney | null; monthly_base_price: PlanMoney | null; annual_base_price: PlanMoney | null;
  additional_licence_monthly_price: PlanMoney | null; contract_required: boolean; core_tender_assistant: "INCLUDED" | "NOT_SPECIFIED";
};
export type PlanCatalogue = { catalogue_version: string; plans: PlanCatalogueEntry[] };
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function numberOrNull(v: unknown): v is number | null { return v === null || Number.isSafeInteger(v) && Number(v) > 0; }
function money(v: unknown): v is PlanMoney | null {
  if (v === null) return true;
  const m = record(v);
  return !!m && Object.keys(m).length === 3 && m.currency === "KES" && typeof m.amount === "string" && /^(0|[1-9][0-9]*)\.[0-9]{2}$/.test(m.amount) && (m.kind === "FIXED" || m.kind === "FROM");
}
function plan(raw: unknown): PlanCatalogueEntry | null {
  const v = record(raw), allowance = record(v?.bid_workspace_allowance), limits = record(v?.limits);
  if (!v || !allowance || !limits || !keys.includes(v.key as PlanKey) || typeof v.name !== "string" || v.name.length < 1 || v.name.length > 100 ||
      (v.catalogue_version !== "PHASE1_V1" && (typeof v.catalogue_version !== "string" || !new RegExp(`^PHASE1_V1-${v.key}-v[1-9]\\d*$`).test(v.catalogue_version))) || !Array.isArray(v.available_cadences) || v.available_cadences.length < 1 || v.available_cadences.length > 2 ||
      !v.available_cadences.every((c) => ["TRIAL", "MONTHLY", "ANNUAL", "CONTRACT"].includes(String(c))) || new Set(v.available_cadences).size !== v.available_cadences.length ||
      !numberOrNull(v.trial_days) || !numberOrNull(v.included_licences) || !numberOrNull(allowance.count) || !["TRIAL_TERM", "BILLING_MONTH", "CONTRACT"].includes(String(allowance.period)) ||
      !numberOrNull(limits.storage_bytes) || !numberOrNull(limits.completed_document_versions) || !numberOrNull(limits.ai_credits) || !numberOrNull(limits.concurrent_ai_jobs) ||
      !["TRIAL_TERM", "BILLING_MONTH", "CONTRACT"].includes(String(limits.reset_period)) || limits.rollover !== false ||
      !money(v.trial_base_price) || !money(v.monthly_base_price) || !money(v.annual_base_price) || !money(v.additional_licence_monthly_price) ||
      typeof v.contract_required !== "boolean" || !["INCLUDED", "NOT_SPECIFIED"].includes(String(v.core_tender_assistant))) return null;
  return v as PlanCatalogueEntry;
}
export function parsePlatformPlanCatalogue(raw: unknown): PlanCatalogue | null {
  const root = record(raw), data = record(root?.data), meta = record(root?.meta);
  if (!root || !data || !meta || typeof meta.trace_id !== "string" || !meta.trace_id || typeof data.catalogue_version !== "string" || !/^PHASE1_V1(?:-(?:BASIC|GROWTH|PROFESSIONAL)-v[1-9]\d*)*$/.test(data.catalogue_version) || data.currency !== "KES" ||
      data.prices_include_vat !== false || data.vat_calculated !== false || data.subscription_activated !== false || data.entitlements_activated !== false ||
      !record(data.ai_credit_policy) || !Array.isArray(data.plans) || data.plans.length !== 5) return null;
  const plans: PlanCatalogueEntry[] = [];
  for (const rawPlan of data.plans) { const entry = plan(rawPlan); if (!entry || plans.some((p) => p.key === entry.key)) return null; plans.push(entry); }
  const version = `PHASE1_V1${plans.filter((p) => p.catalogue_version !== "PHASE1_V1").map((p) => `-${p.catalogue_version.slice("PHASE1_V1-".length)}`).join("")}`;
  if (data.catalogue_version !== version) return null;
  return { catalogue_version: version, plans };
}
