import assert from "node:assert/strict";
import test from "node:test";
import { chromium, expect } from "@playwright/test";
// Synthetic API responses only; no audit records or settings are mutated.
const origin = process.env.BIDORA_WEB_URL ?? "http://127.0.0.1:3000";
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw Error("Use a loopback development origin.");
const record = { id: "11111111-1111-4111-8111-111111111111", administrator_id: "22222222-2222-4222-8222-222222222222", administrator_name: "Example Administrator", action: "PlatformInformationUpdated", module: "ADMINISTRATION", affected_record_id: "33333333-3333-4333-8333-333333333333", affected_record_name: "Platform information", result: "SUCCESSFUL", occurred_at: "2026-01-01T12:00:00Z", ip_address: "192.0.2.20", browser: "Firefox", changes: [{ field: "platform_name", operation: "MODIFIED", before: "Before Platform", after: "After Platform" }, { field: "contact_phone", operation: "DELETED", before: "+254 700 000 001", after: null }, { field: "support_email", operation: "ADDED", before: null, after: "support@example.test" }] };
const envelope = (data) => ({ data, meta: { trace_id: "synthetic-audit" } });
async function open(viewport, row = record, network = true) {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport });
  await page.route("**/api/**", async route => {
    const path = new URL(route.request().url()).pathname;
    let data, status = 200;
    if (path === "/api/session/bootstrap") data = envelope({ csrf_token: "A".repeat(43) });
    else if (path === "/api/session/me") data = envelope({ account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1, capabilities: ["platform.audit.read", ...(network ? ["platform.security.event.read"] : [])], session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now() + 600000).toISOString(), absolute_expires_at: new Date(Date.now() + 3600000).toISOString() }, required_actions: [] });
    else if (path === "/api/session/availability") data = envelope({ maintenance_active: false, maintenance_message: "" });
    else if (path === "/api/platform/audit/actions") data = { ...envelope([row]), meta: { trace_id: "synthetic-audit", next_cursor: null } };
    else if (path === `/api/platform/audit/actions/${row.id}`) data = envelope(row);
    else { status = 503; data = { error: { code: "UNAVAILABLE" } }; }
    await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(data) });
  });
  return { browser, page };
}
for (const [name, viewport] of [["desktop", { width: 1440, height: 900 }], ["mobile", { width: 390, height: 844 }]]) {
  test(`audit table shows names, IP, browser and recorded changes on ${name}`, { timeout: 45000 }, async () => {
    const { browser, page } = await open(viewport);
    try {
      await page.goto(origin + "/platform/audit");
      const table = page.getByRole("table");
      await expect(table.getByText("Example Administrator", { exact: true })).toBeVisible();
      await expect(table.getByText("Platform information", { exact: true })).toHaveCount(1);
      await expect(table.getByText("192.0.2.20", { exact: true })).toHaveCount(1);
      await expect(table.getByText("Firefox", { exact: true })).toHaveCount(1);
      await expect(table.getByText(/Modified platform name; Deleted contact phone; Added support email/)).toHaveCount(1);
      assert.doesNotMatch(await table.innerText(), /[a-f0-9]{8}-[a-f0-9]{4}-/);
      await page.getByRole("button", { name: "Actions for PlatformInformationUpdated" }).click();
      await page.getByRole("menuitem", { name: "View Details", exact: true }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog.getByText("Before: Before Platform", { exact: true })).toBeVisible();
      await expect(dialog.getByText("After: After Platform", { exact: true })).toBeVisible();
      await expect(dialog.getByText("Contact phone · deleted", { exact: true })).toBeVisible();
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
    } finally { await browser.close(); }
  });
}
test("missing historical metadata is not invented and unauthorized metadata is masked", { timeout: 45000 }, async () => {
  const missing = { ...record, administrator_name: null, affected_record_name: null, ip_address: null, browser: null, changes: [] };
  const historical = await open({ width: 1440, height: 900 }, missing);
  try {
    await historical.page.goto(origin + "/platform/audit");
    const table = historical.page.getByRole("table");
    await expect(table.getByText("Unknown administrator", { exact: true })).toBeVisible();
    await expect(table.getByText("Not recorded or expired", { exact: true })).toHaveCount(2);
    await expect(table.getByText("No field details recorded", { exact: true })).toBeVisible();
    assert.doesNotMatch(await table.innerText(), /[a-f0-9]{8}-[a-f0-9]{4}-/);
  } finally { await historical.browser.close(); }
  const restricted = await open({ width: 1440, height: 900 }, record, false);
  try {
    await restricted.page.goto(origin + "/platform/audit");
    const table = restricted.page.getByRole("table");
    await expect(table.getByText("Restricted", { exact: true })).toHaveCount(2);
    await expect(table.getByText("192.0.2.20", { exact: true })).toHaveCount(0);
  } finally { await restricted.browser.close(); }
});
