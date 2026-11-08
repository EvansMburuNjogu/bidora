import assert from "node:assert/strict";
import test from "node:test";
import { parseSupportPage, parseSupportDetail, parseSupportReceipt } from "../src/lib/platform-support-contract.ts";

const id = "11111111-1111-4111-8111-111111111111";
const tenant = "22222222-2222-4222-8222-222222222222";
const meta = { trace_id: "synthetic", next_cursor: null };
const row = { id, tenant_id: tenant, title: "Support case", description: "Control metadata only", state: "OPEN", assigned_to: null, created_by: id, version: 1, created_at: "2026-03-19T12:00:00Z", updated_at: "2026-03-19T12:00:00Z" };
test("support receipts, rows and replies require exact safe control shapes", () => {
  assert.equal(parseSupportPage({ data: [row], meta })?.cases[0].id, id);
  assert.equal(parseSupportPage({ data: [{ ...row, private_document: "secret" }], meta }), null);
  assert.equal(parseSupportPage({ data: [row, row], meta }), null);
  assert.equal(parseSupportDetail({ data: { case: row, messages: [{ id: tenant, actor_id: id, body: "Acknowledged", created_at: "2026-03-19T12:01:00Z" }] }, meta: { trace_id: "synthetic" } }, id)?.messages.length, 1);
  assert.equal(parseSupportDetail({ data: { case: row, messages: [] }, meta: { trace_id: "synthetic" } }, tenant), null);
  assert.deepEqual(parseSupportReceipt({ data: { case_id: id, version: 2, replay: false }, meta: { trace_id: "synthetic" } }), { case_id: id, version: 2, replay: false });
  assert.equal(parseSupportReceipt({ data: { case_id: id, version: 2, replay: false, delivered: true }, meta: { trace_id: "synthetic" } }), null);
});
