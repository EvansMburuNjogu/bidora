import assert from "node:assert/strict";
import test from "node:test";
import { parseOpenAISetting, parseOpenAISettingPage } from "../src/lib/platform-openai-contract.ts";
const id = "11111111-1111-4111-8111-111111111111";
const at = "2026-10-18T12:00:00Z";
const row = { id, setting_kind: "OPENAI", sequence: 1, state: "ACTIVE", version: 2, test_status: "PASSED", test_code: "OPENAI_STRUCTURED_RESPONSE_VERIFIED", created_at: at, tested_at: at, activated_at: at, superseded_at: null,
  configuration: { setting_kind: "OPENAI", endpoint: "https://api.openai.com/v1/responses", model: "approved-model", api_key_reference_configured: true, trust_roots_reference_configured: true, store: false, timeout_seconds: 60, max_concurrency: 2 } };
const meta = { trace_id: id, next_cursor: null };
test("OpenAI setting read rejects provider secrets and unsafe endpoint or storage changes", () => {
  assert.equal(parseOpenAISetting({ data: row, meta })?.configuration.model, "approved-model");
  assert.equal(parseOpenAISettingPage({ data: [row], meta })?.versions.length, 1);
  assert.equal(parseOpenAISetting({ data: { ...row, configuration: { ...row.configuration, api_key: "private" } }, meta }), null);
  assert.equal(parseOpenAISetting({ data: { ...row, configuration: { ...row.configuration, store: true } }, meta }), null);
  assert.equal(parseOpenAISetting({ data: { ...row, configuration: { ...row.configuration, endpoint: "https://other.example" } }, meta }), null);
  assert.equal(parseOpenAISettingPage({ data: [row, row], meta }), null);
});
