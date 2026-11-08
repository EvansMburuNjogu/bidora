import assert from "node:assert/strict";
import test from "node:test";
import { parseManagedPlans, parsePlanReceipt } from "../src/lib/platform-plan-management-contract.ts";
const money = (amount, cents) => ({ currency: "KES", amount, amount_minor_units: cents });
const published = { version: 1, head_version: 1, monthly_price: money("4500.00", 450000), annual_price: money("45000.00", 4500000), additional_licence_monthly_price: money("750.00", 75000), bid_workspaces: 3, included_licences: 3, storage_bytes: 10737418240, document_versions: 250, ai_credits: 5000, concurrent_ai_jobs: 2 };
const plans = ["BASIC", "GROWTH", "PROFESSIONAL"].map((plan_key) => ({ plan_key, published, draft: null, active_version: 1, head_version: 1 }));
test("managed catalogue parses three versioned paid plans and rejects inconsistent versions", () => {
  const response = { data: { plans }, meta: { trace_id: "synthetic" } };
  assert.equal(parseManagedPlans(response)?.length, 3);
  assert.equal(parseManagedPlans({ ...response, data: { plans: [plans[0], plans[0], plans[2]] } }), null);
  assert.equal(parseManagedPlans({ ...response, data: { plans: [{ ...plans[0], published: { ...published, monthly_price: money("12.34", 1224) } }, ...plans.slice(1)] } }), null);
  assert.equal(parseManagedPlans({ ...response, data: { plans: [{ ...plans[0], draft: { ...published, version: 2 } }, ...plans.slice(1)] } })?.[0].draft.version, 2);
  assert.deepEqual(parsePlanReceipt({ data: { plan_key: "BASIC", version: 2, head_version: 3, published: true, replay: false }, meta: { trace_id: "synthetic" } }), { plan_key: "BASIC", version: 2, head_version: 3, published: true });
  assert.equal(parsePlanReceipt({ data: { plan_key: "TRIAL", version: 2, head_version: 3, published: true, replay: false }, meta: { trace_id: "synthetic" } }), null);
});
