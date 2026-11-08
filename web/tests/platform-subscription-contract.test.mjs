import assert from "node:assert/strict";
import test from "node:test";
import { parsePlatformSubscription, parsePlatformSubscriptionReceipt, parsePlatformSubscriptionInventory } from "../src/lib/platform-subscription-contract.ts";
const tenant = "11111111-1111-4111-8111-111111111111", subscription = "22222222-2222-4222-8222-222222222222";
const sub = { subscription_id: subscription, state: "TRIAL_ACTIVE", access_mode: "FULL", version: 1, plan_key: "TRIAL", catalogue_version: "PHASE1_V1", cadence: "TRIAL", commercial_period_end: "2026-05-15T12:00:00Z", cancel_at: null };
test("subscription inventory accepts bounded ordered control rows, not invented totals", () => {
 const row={subscription_id:subscription,tenant_id:tenant,state:"TRIAL_ACTIVE",version:1,plan_key:"TRIAL",cadence:"TRIAL",has_current_term:true,commercial_period_end:"2026-05-15T12:00:00Z",cancel_at:null,updated_at:"2026-05-01T12:00:00Z"};
 const response={data:{subscriptions:[row]},meta:{trace_id:"synthetic",next_cursor:subscription}};
 assert.equal(parsePlatformSubscriptionInventory(response)?.subscriptions[0].tenant_id,tenant);
 assert.equal(parsePlatformSubscriptionInventory({...response,data:{subscriptions:[row,row]}}),null);
 assert.equal(parsePlatformSubscriptionInventory({...response,meta:{trace_id:"synthetic",next_cursor:tenant}}),null);
 assert.equal(parsePlatformSubscriptionInventory({...response,data:{subscriptions:[{...row,has_current_term:false}]}}),null);
});
test("Platform subscription receipt distinguishes a boundary command from payment effects", () => {
 assert.equal(parsePlatformSubscription({ data: { tenant_id: tenant, subscription: sub }, meta: { trace_id: "synthetic" } }, tenant)?.version, 1);
 assert.equal(parsePlatformSubscription({ data: { tenant_id: subscription, subscription: sub }, meta: { trace_id: "synthetic" } }, tenant), null);
 const receipt = { data: { tenant_id: tenant, subscription_id: subscription, state: "TRIAL_ACTIVE", version: 2, cancel_at: "2026-05-15T12:00:00Z", replay: false, payment_executed: false }, meta: { trace_id: "synthetic" } };
 assert.equal(parsePlatformSubscriptionReceipt(receipt, tenant)?.version, 2);
 assert.equal(parsePlatformSubscriptionReceipt({ ...receipt, data: { ...receipt.data, payment_executed: true } }, tenant), null);
});
