import assert from "node:assert/strict";
import test from "node:test";
import { chromium, expect } from "@playwright/test";
import { fixedRoleCapabilities } from "../src/lib/platform-fixed-role-bundles.generated.ts";

// UI checks use synthetic API responses only; no roles or users are changed.
const origin = process.env.BIDORA_WEB_URL ?? "http://127.0.0.1:3000";
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw Error("Use a loopback development origin.");
const envelope = (data) => ({ data, meta: { trace_id: "synthetic-matrix" } });
for (const [name, viewport] of [["desktop", { width: 1280, height: 900 }], ["mobile", { width: 390, height: 844 }]]) {
  test(`complete permissions matrix uses live catalogue grants on ${name}`, { timeout: 45000 }, async () => {
    const browser = await chromium.launch({ headless: true });
    try {
      const page = await browser.newPage({ viewport });
      // Remove one Owner grant from this synthetic live catalogue to prove the
      // UI uses API capabilities rather than assuming Owner always has access.
      const roles = Object.entries(fixedRoleCapabilities).map(([role_key, capabilities]) => ({ role_key, catalogue_version: 1, capabilities: capabilities.filter((key) => role_key !== "PLATFORM_OWNER" || key !== "platform.integration.smtp.test") }));
      await page.route("**/api/**", async (route) => {
        const path = new URL(route.request().url()).pathname;
        let data, status = 200;
        if (path === "/api/session/bootstrap") data = envelope({ csrf_token: "A".repeat(43) });
        else if (path === "/api/session/me") data = envelope({ account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1, capabilities: ["platform.admin.read"], session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now() + 600000).toISOString(), absolute_expires_at: new Date(Date.now() + 3600000).toISOString() }, required_actions: [] });
        else if (path === "/api/session/availability") data = envelope({ maintenance_active: false, maintenance_message: "" });
        else if (path === "/api/platform/roles") data = envelope({ roles, custom_roles_supported: false, tenant_access_implied: false });
        else if (path === "/api/platform/administrators") data = { ...envelope({ administrators: [], effective_owner_count: 0, continuity_warning: true }), meta: { trace_id: "synthetic-matrix", next_cursor: null } };
        else { status = 503; data = { error: { code: "UNAVAILABLE" } }; }
        await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(data) });
      });
      await page.goto(origin + "/platform/settings/access");
      const matrix = page.locator('section[aria-labelledby="permissions-matrix-heading"]');
      const count = new Set(Object.values(fixedRoleCapabilities).flat()).size;
      await expect(matrix.locator("tbody tr").filter({ has: page.locator('th[scope="row"]') })).toHaveCount(count);
      await expect(page.getByText(/Showing fixed role definitions/)).toHaveCount(0);
      for (const label of ["Reactivate accounts", "Use approved support sessions", "View support sessions", "Revoke support sessions", "Test email", "Activate email configuration", "Test AI connection", "Activate AI configuration", "Test payment connection", "Activate payment configuration", "Pause connections", "Roll back connection configuration", "View connection failures", "View migrations", "Manage migrations", "View maintenance settings", "Manage maintenance settings", "Draft retention policy changes"]) {
        await expect(matrix.getByRole("row", { name: label, exact: false }).getByRole("rowheader", { name: label, exact: true })).toHaveCount(1);
      }
      const emailTest = matrix.getByRole("row").filter({ has: page.getByRole("rowheader", { name: "Test email", exact: true }) });
      await expect(emailTest.locator("td").nth(0).getByLabel("Not granted")).toHaveCount(1);
      await expect(emailTest.locator("td").nth(3).getByLabel("Granted", { exact: true })).toHaveCount(1);
      await expect(matrix.getByText("Usage history")).toHaveCount(0);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), "table scroll must not overflow the page");
    } finally { await browser.close(); }
  });
}
