import assert from "node:assert/strict";
import test from "node:test";
import { chromium, expect } from "@playwright/test";

// UI feedback verification only. Every API request is intercepted; no real
// credentials, database writes, email tests or Google/provider calls occur.
const origin = process.env.BIDORA_WEB_URL ?? "http://127.0.0.1:3000";
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw new Error("Use an explicit loopback development web origin.");
const envelope = (data) => ({ data, meta: { trace_id: "synthetic-ui-check" } });
const failure = { error: { code: "REQUEST_FAILED", message: "private diagnostic must not appear", field_errors: [], retryable: true, trace_id: "synthetic" } };
const googleSeed = { id: "019c0000-0000-4000-8000-000000000001", version: 7, enabled: true, allow_signup: false, client_id: "123-managed.apps.googleusercontent.com", allowed_domains: ["example.test"], secret_configured: true, connection_status: "UNTESTED", runtime_configured: true, created_at: "2026-10-31T12:00:00Z" };
const templateSeed = { template_key: "identity.password_recovery", name: "Password recovery", category: "PASSWORD_RECOVERY", channel: "EMAIL", mandatory: true, enabled: true, subject: "Recover your account", body: "Use {{token}} before {{expires_at}}.", version: 7, updated_at: "2026-10-31T12:00:00Z", placeholders: ["platform_name", "token", "expires_at"], test_available: true };
const fulfill = (route, data, status = 200, headers = {}) => route.fulfill({ status, contentType: "application/json", headers, body: JSON.stringify(data) });
async function open(viewport) {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/session/bootstrap") return fulfill(route, envelope({ csrf_token: "A".repeat(43) }));
    if (path === "/api/session/me") return fulfill(route, envelope({ account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1, capabilities: ["platform.admin.read", "platform.integration.read", "platform.governance.approve", "platform.admin.identity.disable", "platform.admin.session.revoke"], session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now() + 600000).toISOString(), absolute_expires_at: new Date(Date.now() + 3600000).toISOString() }, required_actions: [] }));
    return fulfill(route, failure, 503);
  });
  return { browser, page };
}
for (const [name, viewport] of [["desktop", { width: 1280, height: 850 }], ["mobile", { width: 390, height: 844 }]]) {
  test(`Google save feedback, immediate duplicate guard and retry on ${name}`, { timeout: 45000 }, async () => {
    const { browser, page } = await open(viewport);
    let current = structuredClone(googleSeed), calls = 0, release;
    const gate = new Promise((resolve) => { release = resolve; });
    const keys = [];
    try {
      await page.route("**/api/platform/settings/google**", async (route) => {
        if (route.request().method() === "GET") return fulfill(route, envelope(route.request().url().endsWith("/history") ? [current] : current));
        calls++; keys.push(route.request().headers()["idempotency-key"]);
        if (calls === 1) { await gate; return fulfill(route, failure, 503); }
        const payload = route.request().postDataJSON();
        assert.deepEqual(payload.allowed_domains, googleSeed.allowed_domains, "unrelated edits preserve saved domain restrictions");
        current = { ...current, ...payload, version: 8 };
        return fulfill(route, envelope(current));
      });
      await page.goto(origin + "/platform/settings/sso");
      const signup = page.getByRole("switch", { name: "Allow Google Signup", exact: true });
      await expect(signup).toBeEnabled();
      await expect(page.locator("#google-domains")).toHaveCount(0);
      await expect(page.getByText("Allowed Email Domains", { exact: true })).toHaveCount(0);
      await expect(page.locator("[data-sonner-toast]")).toHaveCount(0);
      await signup.click();
      await page.getByRole("button", { name: "Save Changes", exact: true }).click();
      const submit = page.locator('button[type="submit"]');
      await expect(submit).toBeDisabled(); await expect(submit.locator("svg.animate-spin")).toBeVisible();
      await page.getByRole("form", { name: "Google Login and Signup settings" }).evaluate((form) => { form.requestSubmit(); form.requestSubmit(); });
      await expect.poll(() => calls).toBe(1);
      release();
      await expect(page.locator('[data-sonner-toast][data-type="error"]')).toHaveCount(1);
      await expect(signup).toHaveAttribute("aria-checked", "true"); await expect(submit).toBeEnabled();
      await expect(page.getByText("private diagnostic must not appear")).toHaveCount(0);
      await submit.click();
      await expect(page.locator('[data-sonner-toast][data-type="success"]')).toHaveText(/Changes saved successfully/);
      assert.equal(calls, 2); assert.equal(keys[0], keys[1]);
      await expect(submit).toBeDisabled(); await expect(page.locator("[data-sonner-toaster]")).toHaveCount(1);
      const box = await page.locator('[data-sonner-toast][data-type="success"]').boundingBox();
      assert.ok(box && box.x >= 0 && box.x + box.width <= viewport.width + 1, "toast remains within viewport");
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "no horizontal overflow");
    } finally { release(); await browser.close(); }
  });
  test(`template modal keeps values, shows inline fields and closes only on success on ${name}`, { timeout: 45000 }, async () => {
    const { browser, page } = await open(viewport); let current = structuredClone(templateSeed), status = 403, writes = 0;
    try {
      await page.route("**/api/platform/notification-templates**", async (route) => {
        if (route.request().method() === "GET") return fulfill(route, envelope([current]));
        writes++;
        if (status === 400) return fulfill(route, { error: { ...failure.error, field_errors: [{ field: "body", message: "private diagnostic must not appear" }] } }, 400);
        if (status !== 200) return fulfill(route, failure, status);
        current = { ...current, ...route.request().postDataJSON(), version: 8 };
        return fulfill(route, envelope(current));
      });
      await page.goto(origin + "/platform/settings/notifications");
      await page.getByRole("button", { name: "Actions for Password recovery" }).click();
      await page.getByRole("menuitem", { name: "Edit Template", exact: true }).click();
      const dialog = page.getByRole("alertdialog");
      await dialog.getByRole("textbox", { name: "Template Name" }).fill("Updated recovery");
      await dialog.getByRole("button", { name: "Save Changes" }).click();
      await expect(page.locator('[data-sonner-toast][data-type="error"]')).toContainText(/permission/);
      await expect(dialog).toBeVisible(); await expect(dialog.getByRole("textbox", { name: "Template Name" })).toHaveValue("Updated recovery");
      status = 400;
      await dialog.getByRole("button", { name: "Save Changes" }).click();
      await expect(dialog.locator("#template-body-error")).toHaveText("Check this value and try again.");
      await expect(page.getByText("private diagnostic must not appear")).toHaveCount(0);
      await dialog.getByRole("textbox", { name: "Message Body" }).fill("Use {{token}} before {{expires_at}}. Updated instructions.");
      status = 200; await dialog.getByRole("button", { name: "Save Changes" }).click();
      await expect(dialog).toHaveCount(0); await expect(page.getByRole("row", { name: /Updated recovery/ })).toBeVisible();
      await expect(page.locator('[data-sonner-toast][data-type="success"]')).toContainText("Changes saved successfully.");
      assert.equal(writes, 3);
      await expect(page.getByRole("checkbox", { name: /Enable email for Updated recovery/ })).toBeDisabled();
    } finally { await browser.close(); }
  });
}
test("Google pending and JWKS results never become credential-success toasts", { timeout: 45000 }, async () => {
  const { browser, page } = await open({ width: 1280, height: 850 }); let status = "PENDING";
  try {
    await page.route("**/api/platform/settings/google**", (route) => fulfill(route, envelope(route.request().method() === "POST" ? { status } : route.request().url().endsWith("/history") ? [googleSeed] : googleSeed)));
    await page.goto(origin + "/platform/settings/sso"); await page.getByRole("button", { name: "Test Connection", exact: true }).click();
    await expect(page.locator('[data-sonner-toast][data-type="warning"]')).toContainText("pending");
    status = "JWKS_REACHABLE_CREDENTIALS_UNVERIFIED"; await page.getByRole("button", { name: "Test Connection", exact: true }).click();
    await expect(page.locator('[data-sonner-toast][data-type="info"]')).toContainText("verify credentials");
    await expect(page.locator('[data-sonner-toast][data-type="success"]')).toHaveCount(0);
  } finally { await browser.close(); }
});
test("administrator modal preserves failed input and confirms direct access changes only after execution", { timeout: 45000 }, async () => {
  const { browser, page } = await open({ width: 1280, height: 850 });
  const target = "11111111-1111-4111-8111-111111111111", change = "22222222-2222-4222-8222-222222222222", maker = "33333333-3333-4333-8333-333333333333";
  const meta = { trace_id: "synthetic", next_cursor: null }; let status = 503, calls = 0;
  try {
    await page.route("**/api/platform/administrators/**", (route) => fulfill(route, { data: route.request().url().endsWith("/assignment-history") ? { user_id: target, assignments: [] } : { user_id: target, user_version: 2, platform_context_generation: 3, email: "synthetic-owner@example.test", state: "ACTIVE", totp_enrolled: true, assignments: [{ id: change, version: 1, role_key: "PLATFORM_OWNER", state: "ACTIVE", assigned_by: maker, assigned_at: "2026-10-31T12:00:00Z", revoked_by: null, revoked_at: null, reason: "Synthetic assignment", revocation_reason: null }], effective_owner: true, created_at: "2026-10-31T12:00:00Z", updated_at: "2026-10-31T12:00:00Z" }, meta }));
    await page.route("**/api/platform/administrator-changes", (route) => {
      calls++;
      if (status !== 201) return fulfill(route, failure, status);
      return fulfill(route, { data: { id: change, action: "DISABLE_IDENTITY", target_user_id: target, invitation_id: null, role_key: null, assignment_id: null, expected_user_version: 2, expected_target_context_generation: 3, expected_assignment_version: null, reason: "Synthetic access review", decision_reason: null, state: "EXECUTED", version: 1, maker_id: maker, checker_id: null, created_at: "2026-10-31T12:00:00Z", expires_at: null, decided_at: "2026-10-31T12:00:00Z", sessions_revoked: true }, meta }, 201);
    });
    await page.goto(origin + `/platform/users/${target}`);
    await page.getByRole("button", { name: "Disable administrator", exact: true }).click();
    const dialog = page.getByRole("alertdialog");
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("  invalid surrounding spaces  ");
    await dialog.getByRole("button", { name: "Disable administrator", exact: true }).click();
    await expect(dialog.getByRole("alert")).toContainText("without surrounding spaces"); assert.equal(calls, 0);
    await dialog.getByRole("textbox", { name: "Reason", exact: true }).fill("Synthetic access review");
    await dialog.getByRole("button", { name: "Disable administrator", exact: true }).click();
    await expect(page.locator('[data-sonner-toast][data-type="error"]')).toBeVisible();
    await expect(dialog).toBeVisible(); await expect(dialog.getByRole("textbox", { name: "Reason", exact: true })).toHaveValue("Synthetic access review");
    status = 201; await dialog.getByRole("button", { name: "Disable administrator", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page.locator('[data-sonner-toast][data-type="success"]')).toContainText("disabled and sessions revoked");
    await expect(page.getByRole("link", { name: "Review pending changes" })).toHaveCount(0);
    assert.equal(calls, 2);
  } finally { await browser.close(); }
});

test("notification SMTP acceptance is information, not delivered success", { timeout: 45000 }, async () => {
  const { browser, page } = await open({ width: 1280, height: 850 });
  try {
    await page.route("**/api/platform/notification-templates**", (route) => fulfill(route, envelope(route.request().method() === "POST" ? { status: "SMTP_ACCEPTED" } : [templateSeed])));
    await page.goto(origin + "/platform/settings/notifications");
    await page.getByRole("button", { name: "Actions for Password recovery" }).click(); await page.getByRole("menuitem", { name: "Send Test Notification" }).click();
    await expect(page.locator('[data-sonner-toast][data-type="info"]')).toContainText("Recipient delivery is not confirmed");
    await expect(page.locator('[data-sonner-toast][data-type="success"]')).toHaveCount(0);
  } finally { await browser.close(); }
});
