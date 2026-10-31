import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { submissionErrorMessage, submissionFieldErrors, networkSubmissionError, unconfirmedSubmissionError } from "../src/lib/submission-errors.ts";
import { googleDraftErrors, notificationDraftErrors } from "../src/lib/platform-managed-settings-contract.ts";

test("submission failures use safe status messages and bounded Retry-After", () => {
  assert.match(submissionErrorMessage(401), /sign in/);
  assert.match(submissionErrorMessage(403), /permission/);
  assert.match(submissionErrorMessage(409), /record changed/);
  assert.match(submissionErrorMessage(422), /form values/);
  assert.match(submissionErrorMessage(503), /try again/i);
  assert.equal(submissionErrorMessage(200), unconfirmedSubmissionError);
  assert.equal(submissionErrorMessage(429, "45"), "Too many requests. Try again in 45 seconds.");
  const now = Date.parse("2026-10-31T12:00:00Z");
  assert.match(submissionErrorMessage(429, "Sat, 31 Oct 2026 12:01:00 GMT", now), /60 seconds/);
  for (const value of ["-1", "9999999999999", "internal diagnostic", "Sat, 31 Oct 2026 11:00:00 GMT"]) assert.match(submissionErrorMessage(429, value, now), /try again later/i);
  assert.doesNotMatch(networkSubmissionError, /nothing.*changed|still active/i);
});
test("validation projects only allowlisted fields, never upstream messages", async () => {
  const response = new Response(JSON.stringify({ error: { message: "private database diagnostic", field_errors: [{ field: "client_id", message: "secret-canary" }, { field: "api_key", message: "secret-canary" }, { field: "__proto__", message: "secret-canary" }] } }), { status: 400 });
  assert.deepEqual(await submissionFieldErrors(response, ["client_id"]), { client_id: "Check this value and try again." });
  assert.deepEqual(await submissionFieldErrors(new Response("not-json", { status: 422 }), ["body"]), {});
  assert.deepEqual(await submissionFieldErrors(new Response("{}", { status: 503 }), ["body"]), {});
});
test("Google validation identifies individual inputs without echoing secrets", () => {
  const valid = { enabled: true, client_id: "123-managed.apps.googleusercontent.com", client_secret: "", allowed_domains: ["example.test"] };
  assert.deepEqual(googleDraftErrors(valid, false), {});
  assert.ok(googleDraftErrors(valid, true).client_secret);
  assert.ok(googleDraftErrors({ ...valid, client_id: "invalid" }, false).client_id);
  assert.ok(googleDraftErrors({ ...valid, allowed_domains: ["*.example.test"] }, false).allowed_domains);
  assert.ok(googleDraftErrors({ ...valid, client_secret: " private-secret " }, false).client_secret);
});
test("template validation protects mandatory delivery and required placeholders", () => {
  const definition = { template_key: "identity.password_recovery", mandatory: true, placeholders: ["platform_name", "token", "expires_at"] };
  const valid = { name: "Recovery", subject: "Recover your account", body: "Use {{token}} before {{expires_at}}.", enabled: true };
  assert.deepEqual(notificationDraftErrors(definition, valid), {});
  assert.ok(notificationDraftErrors(definition, { ...valid, enabled: false }).enabled);
  assert.ok(notificationDraftErrors(definition, { ...valid, body: "Missing token" }).body);
  assert.ok(notificationDraftErrors(definition, { ...valid, subject: "{{token}}" }).subject);
  assert.ok(notificationDraftErrors(definition, { ...valid, subject: "Bad\r\nHeader" }).subject);
  assert.ok(notificationDraftErrors(definition, { ...valid, name: "é".repeat(41) }).name);
});
test("one shared Toaster and connected action components retain deduplication and immediate locks", async () => {
  const layout = await readFile(new URL("../src/app/layout.tsx", import.meta.url), "utf8");
  assert.equal((layout.match(/<Toaster\b/g) ?? []).length, 1);
  for (const value of ['position="top-right"', "mobileOffset", "duration={5000}", "visibleToasts={3}"]) assert.ok(layout.includes(value), value);
  for (const name of ["google-login-settings", "notifications-settings", "platform-information-form", "platform-security-policy", "administrator-role-change", "administrator-identity-state", "administrator-approvals", "administrator-detail", "smtp-configuration"]) {
    const source = await readFile(new URL(`../src/components/platform/${name}.tsx`, import.meta.url), "utf8");
    for (const value of ["submitting.current", "Loader2Icon", "submissionErrorMessage", "toast.", "id:"]) assert.ok(source.includes(value), `${name}: ${value}`);
  }
});
