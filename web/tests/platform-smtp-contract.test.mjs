import assert from "node:assert/strict";
import test from "node:test";
import { parseSMTPChange, parseSMTPDecision, parseSMTPVersion, parseSMTPVersions } from "../src/lib/platform-smtp-contract.ts";
const id = "11111111-1111-4111-8111-111111111111";
const target = "22222222-2222-4222-8222-222222222222";
const at = "2026-10-18T12:00:00Z";
const config = { setting_kind: "SMTP", host: "smtp.example.test", port: 587, credential_reference_configured: true, trust_roots_reference_configured: true, starttls_required: true, sender_name: "Bidora", sender_address: "no-reply@example.test", reply_to_address: null, connect_timeout_seconds: 10, command_timeout_seconds: 15, delivery_timeout_seconds: 30, max_concurrency: 4 };
const version = { id: target, setting_kind: "SMTP", sequence: 2, state: "TEST_PASSED", version: 2, configuration: config, test_status: "PASSED", test_code: "SMTP_STARTTLS_VERIFIED", created_at: at, tested_at: at, activated_at: null, superseded_at: null };
const change = { id, setting_kind: "SMTP", target_version_id: target, action: "ACTIVATE", state: "PENDING_APPROVAL", version: 1, maker_user_id: target, checker_user_id: null, created_at: at, expires_at: at, decided_at: null };
const meta = { trace_id: id, next_cursor: null };
test("SMTP history parses only redacted typed version records", () => {
  assert.equal(parseSMTPVersions({ data: [version], meta })?.versions[0].configuration.host, "smtp.example.test");
  assert.equal(parseSMTPVersion({ data: version, meta })?.test_status, "PASSED");
  assert.equal(parseSMTPVersions({ data: [version, { ...version, id: id }], meta })?.versions.length, 2);
  assert.equal(parseSMTPVersions({ data: [version, version], meta }), null);
  assert.equal(parseSMTPVersion({ data: { ...version, configuration: { ...config, password: "private" } }, meta }), null);
  assert.equal(parseSMTPVersion({ data: { ...version, setting_kind: "OPENAI" }, meta }), null);
});
test("change lookup and approval require exact change and redacted active version", () => {
  assert.equal(parseSMTPChange({ data: change, meta }, id)?.target_version_id, target);
  assert.equal(parseSMTPChange({ data: change, meta }, target), null);
  assert.equal(parseSMTPDecision({ data: { change: { ...change, state: "APPROVED_EXECUTED", checker_user_id: id, decided_at: at }, active_version: { ...version, state: "ACTIVE", activated_at: at } }, meta }, id)?.state, "APPROVED_EXECUTED");
  assert.equal(parseSMTPDecision({ data: { change, active_version: version }, meta }, id), null);
});
