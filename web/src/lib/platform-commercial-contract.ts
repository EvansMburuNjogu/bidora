export const commercialQueueConfig = {
  invoices: { capability: "platform.invoice.read", states: ["ISSUED", "PAYMENT_PENDING", "PAID", "PARTIALLY_CREDITED", "CREDITED", "VOID", "REVIEW_REQUIRED"], secondary: [] },
  payments: { capability: "platform.payment.read", states: ["CREATED", "PROVIDER_CREATE_PENDING", "AWAITING_CUSTOMER", "PROVIDER_PENDING", "SETTLED", "FAILED", "CANCELLED", "REVERSED", "REFUND_PENDING", "PARTIALLY_REFUNDED", "REFUNDED", "REVIEW_REQUIRED"], secondary: ["NOT_REQUIRED", "DUE", "LEASED", "RETRY_WAIT", "HELD", "COMPLETE"] },
  "financial-adjustments": { capability: "platform.invoice.read", states: ["PENDING_APPROVAL", "APPROVED", "REJECTED", "EXPIRED", "REVIEW_REQUIRED"], secondary: ["ISSUE_CREDIT", "REQUEST_REFUND"] },
  "payment-reversals": { capability: "platform.payment.read", states: ["PENDING", "CONFIRMED", "HELD", "APPLIED"], secondary: [] },
} as const;
export type CommercialQueue = keyof typeof commercialQueueConfig;
export const commercialQueues = Object.keys(commercialQueueConfig) as CommercialQueue[];
export const commercialUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
export type CommercialMoney = { currency: "KES"; amount_minor_units: number; amount: string };
export type CommercialItem = { id: string; tenant_id: string; state: string; amount: CommercialMoney; occurred_at: string; version?: number; fields: { label: string; value: string }[]; detail_id?: string };
export type CommercialPage = { items: CommercialItem[]; next_cursor: string | null };
export type CommercialDetail = { id: string; title: string; state: string; amount: CommercialMoney; fields: { label: string; value: string }[] };

function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function exact(v: Record<string, unknown>, names: readonly string[]): boolean { return Object.keys(v).length === names.length && names.every((name) => Object.hasOwn(v, name)); }
function id(v: unknown): v is string { return typeof v === "string" && commercialUUID.test(v); }
function optionalID(v: unknown): boolean { return v === null || id(v); }
function text(v: unknown): v is string { return typeof v === "string" && v.length > 0 && v.length <= 512; }
function nullableText(v: unknown): boolean { return v === null || text(v); }
function reason(v: unknown): v is string { return typeof v === "string" && v.length > 0 && new TextEncoder().encode(v).byteLength <= 2000; }
function nullableReason(v: unknown): boolean { return v === null || reason(v); }
function instant(v: unknown): v is string { return typeof v === "string" && v.length <= 40 && /^\d{4}-\d\d-\d\dT.*(?:Z|[+-]\d\d:\d\d)$/.test(v) && Number.isFinite(Date.parse(v)); }
function nullableInstant(v: unknown): boolean { return v === null || instant(v); }
function version(v: unknown): v is number { return Number.isSafeInteger(v) && Number(v) > 0; }
function money(v: unknown): v is CommercialMoney {
  const m = record(v);
  return !!m && exact(m, ["currency", "amount_minor_units", "amount"]) && m.currency === "KES" &&
    Number.isSafeInteger(m.amount_minor_units) && Number(m.amount_minor_units) >= 0 && typeof m.amount === "string" &&
    m.amount === `${Math.floor(Number(m.amount_minor_units) / 100)}.${String(Number(m.amount_minor_units) % 100).padStart(2, "0")}`;
}
function envelope(value: unknown): { data: unknown; next_cursor: string | null } | null {
  const r = record(value), m = record(r?.meta);
  if (!r || !m || !exact(r, ["data", "meta"]) || !exact(m, ["trace_id", "next_cursor"]) || !text(m.trace_id) ||
      !(m.next_cursor === null || typeof m.next_cursor === "string" && /^[A-Za-z0-9_-]{1,512}$/.test(m.next_cursor))) return null;
  return { data: r.data, next_cursor: m.next_cursor as string | null };
}
const fields = (r: Record<string, unknown>, names: readonly string[]) => names.map((name) => ({ label: name.replaceAll("_", " "), value: r[name] === null ? "—" : String(r[name]) }));
const base = ["id", "tenant_id", "state", "version", "amount"];
function item(raw: unknown, kind: CommercialQueue): CommercialItem | null {
  const r = record(raw);
  if (!r) return null;
  switch (kind) {
    case "invoices": {
      if (!exact(r, ["id", "tenant_id", "subscription_id", "invoice_number", "state", "version", "purchase_kind", "total", "outstanding", "issued_at", "due_at"]) ||
        !id(r.id) || !id(r.tenant_id) || !id(r.subscription_id) || !text(r.invoice_number) || !commercialQueueConfig.invoices.states.some((s) => s === r.state) || !version(r.version) || !text(r.purchase_kind) || !money(r.total) || !money(r.outstanding) || !instant(r.issued_at) || !instant(r.due_at)) return null;
      return { id: r.id, tenant_id: r.tenant_id, state: r.state as string, version: r.version, amount: r.total, occurred_at: r.issued_at, fields: fields(r, ["invoice_number", "subscription_id", "purchase_kind", "due_at"]).concat({ label: "outstanding", value: `${r.outstanding.amount} KES` }) };
    }
    case "payments": {
      if (!exact(r, [...base, "invoice_id", "reconciliation_state", "reconciliation_pending", "created_at", "updated_at", "reconciled_at"]) ||
        !id(r.id) || !id(r.tenant_id) || !id(r.invoice_id) || !commercialQueueConfig.payments.states.some((s) => s === r.state) || !version(r.version) || !money(r.amount) ||
        !commercialQueueConfig.payments.secondary.some((s) => s === r.reconciliation_state) || typeof r.reconciliation_pending !== "boolean" || !instant(r.created_at) || !instant(r.updated_at) || !nullableInstant(r.reconciled_at)) return null;
      return { id: r.id, tenant_id: r.tenant_id, state: r.state as string, version: r.version, amount: r.amount, occurred_at: r.created_at, fields: fields(r, ["invoice_id", "reconciliation_state", "reconciliation_pending", "updated_at", "reconciled_at"]) };
    }
    case "financial-adjustments": {
      if (!exact(r, [...base, "invoice_id", "payment_id", "maker_user_id", "checker_user_id", "action", "reason_code", "entitlement_disposition", "net", "tax", "created_at", "expires_at", "decided_at"]) ||
        !id(r.id) || !id(r.tenant_id) || !id(r.invoice_id) || !id(r.payment_id) || !id(r.maker_user_id) || !optionalID(r.checker_user_id) || !commercialQueueConfig["financial-adjustments"].states.some((s) => s === r.state) || !commercialQueueConfig["financial-adjustments"].secondary.some((s) => s === r.action) || !version(r.version) || !text(r.reason_code) || !text(r.entitlement_disposition) || !money(r.amount) || !money(r.net) || !money(r.tax) || !instant(r.created_at) || !instant(r.expires_at) || !nullableInstant(r.decided_at)) return null;
      return { id: r.id, tenant_id: r.tenant_id, state: r.state as string, version: r.version, amount: r.amount, occurred_at: r.created_at, detail_id: r.id, fields: fields(r, ["action", "reason_code", "invoice_id", "payment_id", "expires_at"]) };
    }
    case "payment-reversals": {
      if (!exact(r, ["id", "payment_id", "tenant_id", "invoice_id", "actor_user_id", "expected_payment_version", "payment_state", "reconciliation_state", "queue_state", "amount", "requested_at", "applied_at", "hold_state"]) ||
        !id(r.id) || !id(r.payment_id) || !id(r.tenant_id) || !id(r.invoice_id) || !id(r.actor_user_id) || !version(r.expected_payment_version) || !text(r.payment_state) || !text(r.reconciliation_state) || !commercialQueueConfig["payment-reversals"].states.some((s) => s === r.queue_state) || !money(r.amount) || !instant(r.requested_at) || !nullableInstant(r.applied_at) || !nullableText(r.hold_state)) return null;
      return { id: r.id, tenant_id: r.tenant_id, state: r.queue_state as string, amount: r.amount, occurred_at: r.requested_at, detail_id: r.queue_state === "APPLIED" ? r.payment_id as string : undefined, fields: fields(r, ["payment_id", "payment_state", "reconciliation_state", "hold_state", "applied_at"]) };
    }
  }
}
export function parseCommercialPage(raw: unknown, kind: CommercialQueue): CommercialPage | null {
  const e = envelope(raw);
  if (!e || !Array.isArray(e.data) || e.data.length > 100) return null;
  const items: CommercialItem[] = [], seen = new Set<string>();
  for (const value of e.data) { const parsed = item(value, kind); if (!parsed || seen.has(parsed.id.toLowerCase())) return null; seen.add(parsed.id.toLowerCase()); items.push(parsed); }
  return { items, next_cursor: e.next_cursor };
}
export function parseCommercialDetail(raw: unknown, kind: "financial-adjustments" | "payment-reversals", requested: string): CommercialDetail | null {
  const e = envelope(raw), r = record(e?.data);
  if (!e || e.next_cursor !== null || !r) return null;
  if (kind === "financial-adjustments") {
    if (!exact(r, ["id", "tenant_id", "invoice_id", "payment_id", "action", "state", "version", "reason_code", "entitlement_disposition", "proposal_reason", "decision_reason", "maker_user_id", "checker_user_id", "amount", "net", "tax", "created_at", "expires_at", "decided_at", "credit_note_id", "credit_number", "credit_note_state", "refund_operation_id", "refund_state"]) ||
        !id(r.id) || r.id.toLowerCase() !== requested.toLowerCase() || !id(r.tenant_id) || !id(r.invoice_id) || !id(r.payment_id) || !id(r.maker_user_id) || !optionalID(r.checker_user_id) || !optionalID(r.credit_note_id) || !optionalID(r.refund_operation_id) || !version(r.version) || !commercialQueueConfig["financial-adjustments"].states.some((s) => s === r.state) || !commercialQueueConfig["financial-adjustments"].secondary.some((s) => s === r.action) || !text(r.reason_code) || !text(r.entitlement_disposition) || !reason(r.proposal_reason) || !nullableReason(r.decision_reason) || !nullableText(r.credit_number) || !nullableText(r.credit_note_state) || !nullableText(r.refund_state) || !money(r.amount) || !money(r.net) || !money(r.tax) || !instant(r.created_at) || !instant(r.expires_at) || !nullableInstant(r.decided_at)) return null;
    return { id: r.id, title: "Financial adjustment", state: r.state as string, amount: r.amount, fields: fields(r, ["tenant_id", "invoice_id", "payment_id", "action", "reason_code", "entitlement_disposition", "proposal_reason", "decision_reason", "maker_user_id", "checker_user_id", "net", "tax", "created_at", "expires_at", "decided_at", "credit_note_id", "credit_number", "credit_note_state", "refund_operation_id", "refund_state"]).map((f) => f.label === "net" || f.label === "tax" ? { ...f, value: `${(r[f.label] as CommercialMoney).amount} KES` } : f) };
  }
  if (!exact(r, ["payment_id", "tenant_id", "invoice_id", "subscription_id", "settlement_observation_id", "reversal_observation_id", "access_hold_id", "payment_state", "invoice_state", "subscription_state", "access_mode", "hold_state", "amount", "provider_observed_at", "recorded_at", "applied_at"]) ||
      !id(r.payment_id) || r.payment_id.toLowerCase() !== requested.toLowerCase() || !["tenant_id", "invoice_id", "subscription_id", "settlement_observation_id", "reversal_observation_id", "access_hold_id"].every((name) => id(r[name])) || !["payment_state", "invoice_state", "subscription_state", "access_mode", "hold_state"].every((name) => text(r[name])) || !money(r.amount) || !nullableInstant(r.provider_observed_at) || !instant(r.recorded_at) || !instant(r.applied_at)) return null;
  return { id: r.payment_id, title: "Payment reversal", state: r.payment_state as string, amount: r.amount, fields: fields(r, ["tenant_id", "invoice_id", "subscription_id", "settlement_observation_id", "reversal_observation_id", "access_hold_id", "invoice_state", "subscription_state", "access_mode", "hold_state", "provider_observed_at", "recorded_at", "applied_at"]) };
}
