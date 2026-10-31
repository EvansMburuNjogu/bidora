import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { parseGoogleSettings, parseNotificationTemplate, parseTemplateList, settingsData } from "../src/lib/platform-managed-settings-contract.ts";

const google = { id: "019c0000-0000-4000-8000-000000000001", version: 2, enabled: true, allow_signup: false, client_id: "123-managed.apps.googleusercontent.com", allowed_domains: ["example.test"], secret_configured: true, connection_status: "JWKS_REACHABLE_CREDENTIALS_UNVERIFIED", runtime_configured: true, created_at: "2026-10-30T12:00:00Z" };
const template = { template_key: "identity.password_recovery", name: "Password recovery", category: "PASSWORD_RECOVERY", channel: "EMAIL", mandatory: true, enabled: true, subject: "Recover your account", body: "Use {{token}} before {{expires_at}}", version: 2, updated_at: "2026-10-30T12:00:00Z", placeholders: ["platform_name", "token", "expires_at"], test_available: false };
test("Google metadata is exact, redacted and does not claim credential verification", () => {
  assert.deepEqual(parseGoogleSettings(google), google);
  for (const change of [{ client_secret: "must-not-be-returned" }, { ciphertext: "must-not-be-returned" }, { version: 0 }, { version: Number.MAX_SAFE_INTEGER + 1 }, { connection_status: "CONNECTED" }, { created_at: "not-a-date" }, { allowed_domains: ["https://example.test"] }]) assert.equal(parseGoogleSettings({ ...google, ...change }), null);
  assert.equal(settingsData({ data: google, meta: { trace_id: "synthetic" }, client_secret: "must-not-be-returned" }), null);
});
test("notification projection keeps mandatory email enabled and rejects unsupported channels", () => {
  assert.deepEqual(parseNotificationTemplate(template), template);
  for (const change of [{ enabled: false }, { channel: "SMS" }, { recipient_email: "must-not-be-returned@example.test" }, { version: -1 }, { placeholders: ["<script>"] }, { template_key: "../tenant-private" }, { updated_at: "invalid" }]) assert.equal(parseNotificationTemplate({ ...template, ...change }), null);
  assert.deepEqual(parseTemplateList({ data: [template], meta: { trace_id: "synthetic" } }), [template]);
  assert.equal(parseTemplateList({ data: Array(101).fill(template), meta: { trace_id: "synthetic" } }), null);
});
test("managed BFF keeps exact context, bounded bodies and fixed resource targets", async () => {
  const source = await readFile(new URL("../src/lib/server/managed-settings-proxy.ts", import.meta.url), "utf8");
  for (const invariant of ["X-Bidora-Platform-Context-Generation", "request.headers.get(\"origin\") !== browser", "16_384", "reader.cancel()", "If-Match", "Idempotency-Key", "backendCookieHeader", "browserSetCookie", "redirect: \"manual\"", "cache: \"no-store\"", "AbortSignal.timeout(20_000)"]) assert.ok(source.includes(invariant), invariant);
  assert.ok(!source.includes("localStorage"));
});
