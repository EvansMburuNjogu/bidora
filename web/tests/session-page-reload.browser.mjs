import assert from "node:assert/strict";
import { chromium } from "@playwright/test";

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  let bootstrapReads = 0;
  let tenantReads = 0;
  let beforeConflictCheck = 0;
  await page.route("**/api/**", route => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/session/bootstrap") {
      bootstrapReads++;
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { csrf_token: "A".repeat(43) }, meta: { trace_id: "fixture" } }) });
    }
    if (path === "/api/session/me") return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: {
      account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1,
      capabilities: ["platform.tenant.read"], required_actions: [], session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now() + 900_000).toISOString(), absolute_expires_at: new Date(Date.now() + 3600_000).toISOString() },
    }, meta: { trace_id: "fixture" } }) });
    if (path === "/api/platform/tenants") {
      tenantReads++;
      beforeConflictCheck = bootstrapReads;
      return route.fulfill({ status: 409, contentType: "application/json", body: "{}" });
    }
    return route.fulfill({ status: 503, contentType: "application/json", body: "{}" });
  });
  await page.goto((process.env.BIDORA_TEST_WEB_ORIGIN || "http://127.0.0.1:3000") + "/platform");
  await page.getByRole("heading", { name: "Overview", exact: true }).waitFor();
  await page.getByText("Unavailable for this session.", { exact: true }).waitFor();
  await page.waitForTimeout(2000);
  assert.equal(tenantReads, 1, "conflicting page request must not remount and repeat");
  assert.equal(bootstrapReads, beforeConflictCheck + 1, "page conflict should trigger one session recheck, not a refresh loop");
  assert.equal(await page.getByRole("status", { name: "Loading", exact: true }).count(), 0);
  console.log("PASS: Platform page conflict triggers one background session check without remount/reload loops (intercepted APIs only)");
} finally { await browser.close(); }
