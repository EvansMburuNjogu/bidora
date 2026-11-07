import assert from "node:assert/strict";
import test from "node:test";
import { parsePlatformInformation } from "../src/lib/platform-information-contract.ts";
import { parsePlatformSessionPolicy } from "../src/lib/platform-session-policy-contract.ts";

const information = { data: { platform_name: "Bidora Pilot", contact_phone: "+254 700 000 001", support_email: "support@example.test", website: "https://example.test", version: 2, updated_at: "2026-10-30T12:00:00Z" }, meta: { trace_id: "synthetic", next_cursor: null } };
test("platform identity parser accepts current shared contacts and rejects unsafe or extra fields", () => {
  assert.deepEqual(parsePlatformInformation(information), information.data);
  for (const change of [{ platform_name: "name\nheader" }, { contact_phone: "javascript:bad" }, { website: "javascript:alert(1)" }, { website: "https://user:password@example.test" }, { support_email: "mail@example.test?subject=bad" }, { api_key: "must-not-be-returned" }]) assert.equal(parsePlatformInformation({ ...information, data: { ...information.data, ...change } }), null);
});
test("session policy parser preserves hard security ceilings and rejects unsupported controls", () => {
  const data = { idle_seconds: 300, absolute_seconds: 3600, security: { failed_login_attempts: 5, failure_window_seconds: 900, lockout_seconds: 900, minimum_password_bytes: 12, password_reset_seconds: 1800, allow_multiple_sessions: true, admin_idle_seconds: 604800, admin_absolute_seconds: 604800 }, version: 2 };
  assert.deepEqual(parsePlatformSessionPolicy({ data, meta: { trace_id: "synthetic", next_cursor: null } }), data);
  for (const change of [{ idle_seconds: 3600 }, { absolute_seconds: 604800 }, { mfa_required: false }, { version: 0 }]) assert.equal(parsePlatformSessionPolicy({ data: { ...data, ...change }, meta: { trace_id: "synthetic" } }), null);
});
