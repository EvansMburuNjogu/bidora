import assert from "node:assert/strict";
import test from "node:test";
import { chromium, expect } from "@playwright/test";

// Browser workflow checks with intercepted APIs only: no real settings writes.
const origin = process.env.BIDORA_WEB_URL ?? "http://127.0.0.1:3000";
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw Error("Use a loopback development origin.");
const envelope = (data) => ({ data, meta: { trace_id: "synthetic-information" } });
const seed = { platform_name: "Saved Platform", contact_phone: "+254 700 000 001", support_email: "help@example.test", website: "https://example.test", version: 7, updated_at: "2026-01-01T12:00:00Z" };
const failure = { error: { code: "UNAVAILABLE", message: "private diagnostic", field_errors: [], retryable: true } };
async function open(viewport = { width: 1280, height: 900 }, capabilities = ["platform.admin.read", "platform.governance.approve"]) {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport });
  const state = { saved: { ...seed }, readStatus: 200, writeStatus: 200, failReadAfterSave: false, writes: [] };
  await page.route("**/api/**", async (route) => {
    const request = route.request(), path = new URL(request.url()).pathname;
    let data, status = 200;
    if (path === "/api/session/bootstrap") data = envelope({ csrf_token: "A".repeat(43) });
    else if (path === "/api/session/me") data = envelope({ account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1, capabilities, session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now() + 600000).toISOString(), absolute_expires_at: new Date(Date.now() + 3600000).toISOString() }, required_actions: [] });
    else if (path === "/api/session/availability") data = envelope({ maintenance_active: false, maintenance_message: "" });
    else if (path === "/api/session/countries") data = envelope({ countries: [{ country_code: "KE", name: "Kenya", currency: "KES", language: "en", timezone: "Africa/Nairobi", locale: "en-KE", enabled: true, is_default: true, version: 1, updated_at: seed.updated_at }], default_country: "KE", billing_currency: "KES", organization_registration_enabled: true });
    else if (path === "/api/session/platform-information") data = envelope(state.saved);
    else if (path === "/api/platform/information") {
      if (request.method() === "PUT") {
        state.writes.push({ body: request.postDataJSON(), headers: request.headers() });
        status = state.writeStatus;
        if (status === 200) {
          state.saved = { ...state.saved, ...request.postDataJSON(), version: state.saved.version + 1 };
          data = envelope(state.saved);
          if (state.failReadAfterSave) state.readStatus = 503;
        } else data = failure;
      } else { status = state.readStatus; data = status === 200 ? envelope(state.saved) : failure; }
    } else { status = 503; data = failure; }
    await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(data) });
  });
  return { browser, page, state };
}
async function assertValues(page, value) {
  for (const [id, key] of [["platform-name", "platform_name"], ["platform-website", "website"], ["platform-support-email", "support_email"], ["platform-phone", "contact_phone"]]) {
    await expect(page.locator(`#${id}`)).toHaveValue(value[key] ?? "");
  }
}
for (const [name, viewport] of [["desktop", { width: 1280, height: 900 }], ["mobile", { width: 390, height: 844 }]]) {
  test(`platform information retrieves, saves, refreshes and reopens on ${name}`, { timeout: 45000 }, async () => {
    const { browser, page, state } = await open(viewport);
    try {
      await page.goto(origin + "/platform/settings/platform");
      await assertValues(page, seed);
      const inputs = page.getByRole("form", { name: "Platform information" }).locator("input");
      assert.equal(await inputs.count(), 8);
      for (const input of await inputs.all()) await expect(input).toHaveAttribute("placeholder", /\S/);
      await expect(page.getByText("Manage countries and localisation", { exact: true })).toHaveCount(0);
      await expect(page.locator("#default_country")).toHaveValue("Kenya");
      await expect(page.locator("#default_currency")).toHaveValue("KES");
      await expect(page.getByRole("button", { name: "Save Changes", exact: true })).toBeDisabled();
      await page.locator("#platform-name").fill("Updated Platform");
      await page.locator("#platform-website").fill("https://updated.example.test");
      await page.locator("#platform-support-email").fill("support@updated.example.test");
      await page.locator("#platform-phone").fill("+254 711 222 333");
      await page.getByRole("button", { name: "Save Changes", exact: true }).click();
      await expect(page.locator('[data-sonner-toast][data-type="success"]')).toContainText("Changes saved successfully.");
      await assertValues(page, state.saved);
      assert.equal(state.writes.length, 1);
      assert.equal(state.writes[0].headers["if-match"], '"7"');
      assert.equal(state.writes[0].headers["x-bidora-platform-context-generation"], "1");
      assert.equal(state.writes[0].headers["x-csrf-token"], "A".repeat(43));
      assert.ok(state.writes[0].headers["idempotency-key"]);
      assert.deepEqual(Object.keys(state.writes[0].body).sort(), ["contact_phone", "platform_name", "support_email", "website"]);
      await page.reload();
      await assertValues(page, state.saved);
      await page.goto(origin + "/platform/settings");
      await page.goto(origin + "/platform/settings/platform");
      await assertValues(page, state.saved);
      await page.locator("#platform-name").fill("Unsaved edit");
      await page.getByRole("button", { name: "Cancel", exact: true }).click();
      await assertValues(page, state.saved);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
    } finally { await browser.close(); }
  });
}
test("failed loading never shows fake defaults and retry retrieves saved values", { timeout: 45000 }, async () => {
  const { browser, page, state } = await open();
  try {
    state.readStatus = 503;
    await page.goto(origin + "/platform/settings/platform");
    await expect(page.locator("main.app-page").getByRole("alert")).toContainText("Saved details unavailable.");
    await expect(page.getByRole("form", { name: "Platform information" })).toHaveCount(0);
    await expect(page.locator("#platform-name")).toHaveCount(0);
    await expect(page.getByText("private diagnostic")).toHaveCount(0);
    state.readStatus = 200;
    await page.getByRole("button", { name: "Retry loading saved details" }).click();
    await assertValues(page, seed);
    state.writeStatus = 503;
    await page.locator("#platform-name").fill("Keep my edit");
    await page.getByRole("button", { name: "Save Changes", exact: true }).click();
    await expect(page.locator('[data-sonner-toast][data-type="error"]')).toBeVisible();
    await expect(page.locator("#platform-name")).toHaveValue("Keep my edit");
    await expect(page.locator('[data-sonner-toast][data-type="success"]')).toHaveCount(0);
  } finally { await browser.close(); }
});
test("access errors hide inputs and invalid edits never submit", { timeout: 45000 }, async () => {
  const { browser, page, state } = await open();
  try {
    state.readStatus = 403;
    await page.goto(origin + "/platform/settings/platform");
    await expect(page.locator("main.app-page").getByRole("alert")).toContainText("Recent verification is required");
    await expect(page.locator("#platform-name")).toHaveCount(0);
    state.readStatus = 200;
    await page.getByRole("button", { name: "Retry loading saved details" }).click();
    await assertValues(page, seed);
    await page.locator("#platform-website").fill("http://unsafe.example.test");
    await page.locator("#platform-website").blur();
    await expect(page.locator("#website-error")).toBeVisible();
    await expect(page.getByRole("button", { name: "Save Changes", exact: true })).toBeDisabled();
    assert.equal(state.writes.length, 0);
    await page.getByRole("button", { name: "Cancel", exact: true }).click();
    await assertValues(page, seed);
  } finally { await browser.close(); }
});
test("accepted write retains its values and version when confirmation fails", { timeout: 45000 }, async () => {
  const { browser, page, state } = await open();
  try {
    state.failReadAfterSave = true;
    await page.goto(origin + "/platform/settings/platform");
    await assertValues(page, seed);
    await page.locator("#platform-name").fill("Accepted Platform");
    await page.getByRole("button", { name: "Save Changes", exact: true }).click();
    await expect(page.locator("main.app-page").getByRole("alert")).toContainText("Changes were accepted");
    await assertValues(page, state.saved);
    await expect(page.locator('[data-sonner-toast][data-type="success"]')).toHaveCount(0);
    state.readStatus = 200;
    await page.getByRole("button", { name: "Reload saved details" }).click();
    await assertValues(page, state.saved);
    state.failReadAfterSave = false;
    await page.locator("#platform-name").fill("Confirmed Platform");
    await page.getByRole("button", { name: "Save Changes", exact: true }).click();
    await expect(page.locator('[data-sonner-toast][data-type="success"]')).toBeVisible();
    assert.equal(state.writes[1].headers["if-match"], '"8"');
  } finally { await browser.close(); }
});
test("conflicts offer reload and read-only administrators see saved values", { timeout: 45000 }, async () => {
  const { browser, page, state } = await open();
  try {
    await page.goto(origin + "/platform/settings/platform");
    await assertValues(page, seed);
    state.writeStatus = 409;
    await page.locator("#platform-name").fill("Conflicting edit");
    await page.getByRole("button", { name: "Save Changes", exact: true }).click();
    await expect(page.locator("main.app-page").getByRole("alert")).toContainText("These settings have changed.");
    state.saved = { ...state.saved, platform_name: "Another saved edit", version: 8 };
    await page.getByRole("button", { name: "Reload saved details" }).click();
    await assertValues(page, state.saved);
  } finally { await browser.close(); }
  const readOnly = await open(undefined, ["platform.admin.read"]);
  try {
    await readOnly.page.goto(origin + "/platform/settings/platform");
    await assertValues(readOnly.page, seed);
    await expect(readOnly.page.locator("#platform-name")).toBeDisabled();
    await expect(readOnly.page.getByRole("button", { name: "Save Changes", exact: true })).toBeDisabled();
    assert.equal(readOnly.state.writes.length, 0);
  } finally { await readOnly.browser.close(); }
});
