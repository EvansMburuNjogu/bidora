import assert from "node:assert/strict";
import test from "node:test";
import { parseCommercialPage, parseCommercialDetail } from "../src/lib/platform-commercial-contract.ts";
const id = "11111111-1111-4111-8111-111111111111";
const other = "22222222-2222-4222-8222-222222222222";
const at = "2026-10-18T12:00:00Z";
const amount = { currency: "KES", amount_minor_units: 1250, amount: "12.50" };
const meta = { trace_id: id, next_cursor: null };
const invoice = { id, tenant_id: other, subscription_id: other, invoice_number: "SYN-1", state: "ISSUED", version: 1, purchase_kind: "INITIAL", total: amount, outstanding: amount, issued_at: at, due_at: at };
const payment = { id, tenant_id: other, invoice_id: other, state: "SETTLED", version: 2, amount, reconciliation_state: "COMPLETE", reconciliation_pending: false, created_at: at, updated_at: at, reconciled_at: at };
const adjustment = { id, tenant_id: other, invoice_id: other, payment_id: other, maker_user_id: other, checker_user_id: null, action: "REQUEST_REFUND", state: "PENDING_APPROVAL", reason_code: "SYNTHETIC", entitlement_disposition: "NONE", version: 1, amount, net: amount, tax: { currency: "KES", amount_minor_units: 0, amount: "0.00" }, created_at: at, expires_at: at, decided_at: null };
const reversal = { id, payment_id: other, tenant_id: other, invoice_id: other, actor_user_id: other, expected_payment_version: 2, payment_state: "REVERSED", reconciliation_state: "HELD", queue_state: "PENDING", amount, requested_at: at, applied_at: null, hold_state: null };

test("four audited queues parse exact safe fields and distinguish reversal payment locator", () => {
  for (const [kind, row] of [["invoices", invoice], ["payments", payment], ["financial-adjustments", adjustment], ["payment-reversals", reversal]]) {
    const parsed = parseCommercialPage({ data: [row], meta }, kind);
    assert.equal(parsed?.items[0].id, id);
    assert.equal(parsed?.items[0].amount.amount, "12.50");
    assert.equal(parseCommercialPage({ data: [{ ...row, database_name: "private" }], meta }, kind), null);
    assert.equal(parseCommercialPage({ data: [row, row], meta }, kind), null);
  }
  assert.equal(parseCommercialPage({ data: [reversal], meta }, "payment-reversals")?.items[0].detail_id, undefined);
  assert.equal(parseCommercialPage({ data: [{ ...reversal, queue_state: "APPLIED", applied_at: at }], meta }, "payment-reversals")?.items[0].detail_id, other);
});
test("financial details require exact requested ID, amount and shape", () => {
  const detail = { ...adjustment, proposal_reason: "Review", decision_reason: null, credit_note_id: null, credit_number: null, credit_note_state: null, refund_operation_id: null, refund_state: null };
  assert.equal(parseCommercialDetail({ data: detail, meta }, "financial-adjustments", id)?.state, "PENDING_APPROVAL");
  assert.equal(parseCommercialDetail({ data: detail, meta }, "financial-adjustments", other), null);
  assert.equal(parseCommercialDetail({ data: { ...detail, amount: { ...amount, amount: "1250" } }, meta }, "financial-adjustments", id), null);
  assert.equal(parseCommercialDetail({ data: detail, meta: { ...meta, next_cursor: "not-detail" } }, "financial-adjustments", id), null);
  const reversed = { payment_id: other, tenant_id: id, invoice_id: id, subscription_id: id, settlement_observation_id: id,
    reversal_observation_id: id, access_hold_id: id, payment_state: "REVERSED", invoice_state: "REVIEW_REQUIRED", subscription_state: "SUSPENDED",
    access_mode: "RETAINED_READ_ONLY", hold_state: "ACTIVE", amount, provider_observed_at: null, recorded_at: at, applied_at: at };
  assert.equal(parseCommercialDetail({ data: reversed, meta }, "payment-reversals", other)?.title, "Payment reversal");
  assert.equal(parseCommercialDetail({ data: reversed, meta }, "payment-reversals", id), null);
});
