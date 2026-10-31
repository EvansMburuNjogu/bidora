import type { CommercialMoney, CommercialQueue } from "./platform-commercial-contract";

const commercialUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const states = {
  invoices: ["ISSUED", "PAYMENT_PENDING", "PAID", "PARTIALLY_CREDITED", "CREDITED", "VOID", "REVIEW_REQUIRED"],
  payments: ["CREATED", "PROVIDER_CREATE_PENDING", "AWAITING_CUSTOMER", "PROVIDER_PENDING", "SETTLED", "FAILED", "CANCELLED", "REVERSED", "REFUND_PENDING", "PARTIALLY_REFUNDED", "REFUNDED", "REVIEW_REQUIRED"],
  adjustments: ["PENDING_APPROVAL", "APPROVED", "REJECTED", "EXPIRED", "REVIEW_REQUIRED"],
  reversals: ["PENDING", "CONFIRMED", "HELD", "APPLIED"],
};
function parseCommercialMoney(v: unknown): v is CommercialMoney {
  const m = record(v);
  return !!m && exact(m, ["currency", "amount_minor_units", "amount"]) && m.currency === "KES" && Number.isSafeInteger(m.amount_minor_units) && Number(m.amount_minor_units) >= 0 && typeof m.amount === "string" &&
    m.amount === `${Math.floor(Number(m.amount_minor_units) / 100)}.${String(Number(m.amount_minor_units) % 100).padStart(2, "0")}`;
}

export type PlatformDashboardItem = { id: string; tenant_id: string; state: string; amount: CommercialMoney; occurred_at: string; label: string; detail_href: string | null };
export type PlatformDashboardWindow = { items: PlatformDashboardItem[]; has_more: boolean };
export type PlatformBillingDashboard = { observed_at: string; windows: Record<CommercialQueue, PlatformDashboardWindow> };
function record(value: unknown): Record<string, unknown> | null { return value !== null && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : null; }
function exact(value: Record<string, unknown>, fields: readonly string[]): boolean { return Object.keys(value).length === fields.length && fields.every((field) => Object.hasOwn(value, field)); }
function id(value: unknown): value is string { return typeof value === "string" && commercialUUID.test(value); }
function text(value: unknown): value is string { return typeof value === "string" && value.length > 0 && value.length <= 512; }
function instant(value: unknown): value is string { return typeof value === "string" && value.length <= 40 && /^\d{4}-\d\d-\d\dT.*(?:Z|[+-]\d\d:\d\d)$/.test(value) && Number.isFinite(Date.parse(value)); }
function positive(value: unknown): boolean { return Number.isSafeInteger(value) && Number(value) > 0; }
function item(raw: unknown, queue: CommercialQueue): PlatformDashboardItem | null {
  const row = record(raw);
  if (!row || !id(row.id) || !id(row.tenant_id)) return null;
  switch (queue) {
    case "invoices":
      if (!exact(row, ["id", "tenant_id", "invoice_number", "state", "version", "total", "outstanding", "issued_at"]) || !text(row.invoice_number) || !states.invoices.includes(String(row.state)) || !positive(row.version) || !parseCommercialMoney(row.total) || !parseCommercialMoney(row.outstanding) || !instant(row.issued_at)) return null;
      return { id: row.id, tenant_id: row.tenant_id, state: row.state as string, amount: row.total, occurred_at: row.issued_at, label: row.invoice_number, detail_href: null };
    case "payments":
      if (!exact(row, ["id", "tenant_id", "invoice_id", "state", "reconciliation_state", "version", "amount", "updated_at"]) || !id(row.invoice_id) || !states.payments.includes(String(row.state)) || !["NOT_REQUIRED", "DUE", "LEASED", "RETRY_WAIT", "HELD", "COMPLETE"].includes(String(row.reconciliation_state)) || !positive(row.version) || !parseCommercialMoney(row.amount) || !instant(row.updated_at)) return null;
      return { id: row.id, tenant_id: row.tenant_id, state: row.state as string, amount: row.amount, occurred_at: row.updated_at, label: `Payment ${row.id}`, detail_href: null };
    case "financial-adjustments":
      if (!exact(row, ["id", "tenant_id", "invoice_id", "action", "state", "reason_code", "version", "amount", "created_at", "expires_at"]) || !id(row.invoice_id) || !["ISSUE_CREDIT", "REQUEST_REFUND"].includes(String(row.action)) || !states.adjustments.includes(String(row.state)) || !text(row.reason_code) || !positive(row.version) || !parseCommercialMoney(row.amount) || !instant(row.created_at) || !instant(row.expires_at)) return null;
      return { id: row.id, tenant_id: row.tenant_id, state: row.state as string, amount: row.amount, occurred_at: row.created_at, label: `${String(row.action).replaceAll("_", " ")} · ${row.reason_code}`, detail_href: `/platform/billing/financial-adjustments/${row.id}` };
    case "payment-reversals":
      if (!exact(row, ["id", "tenant_id", "payment_id", "payment_state", "queue_state", "hold_state", "amount", "requested_at", "applied_at"]) || !id(row.payment_id) || !text(row.payment_state) || !states.reversals.includes(String(row.queue_state)) || !(row.hold_state === null || text(row.hold_state)) || !parseCommercialMoney(row.amount) || !instant(row.requested_at) || !(row.applied_at === null || instant(row.applied_at))) return null;
      return { id: row.id, tenant_id: row.tenant_id, state: row.queue_state as string, amount: row.amount, occurred_at: row.requested_at, label: `Payment ${row.payment_id}`, detail_href: row.queue_state === "APPLIED" ? `/platform/billing/payment-reversals/${row.payment_id}` : null };
  }
}
function window(raw: unknown, queue: CommercialQueue): PlatformDashboardWindow | null {
  const value = record(raw);
  if (!value || !exact(value, ["items", "has_more"]) || !Array.isArray(value.items) || value.items.length > 5 || typeof value.has_more !== "boolean") return null;
  const seen = new Set<string>(), items: PlatformDashboardItem[] = [];
  for (const rawItem of value.items) { const parsed = item(rawItem, queue); if (!parsed || seen.has(parsed.id.toLowerCase())) return null; items.push(parsed); seen.add(parsed.id.toLowerCase()); }
  return { items, has_more: value.has_more };
}
export function parsePlatformBillingDashboard(raw: unknown): PlatformBillingDashboard | null {
  const root = record(raw), meta = record(root?.meta), data = record(root?.data);
  if (!root || !meta || !data || !exact(root, ["data", "meta"]) || !exact(meta, ["trace_id", "next_cursor"]) || !text(meta.trace_id) || meta.next_cursor !== null ||
      !exact(data, ["observed_at", "invoices", "payments", "adjustments", "reversals"]) || !instant(data.observed_at)) return null;
  const invoices = window(data.invoices, "invoices"), payments = window(data.payments, "payments"), adjustments = window(data.adjustments, "financial-adjustments"), reversals = window(data.reversals, "payment-reversals");
  if (!invoices || !payments || !adjustments || !reversals) return null;
  return { observed_at: data.observed_at, windows: { invoices, payments, "financial-adjustments": adjustments, "payment-reversals": reversals } };
}
