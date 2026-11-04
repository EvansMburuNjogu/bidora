import assert from "node:assert/strict";
import { chromium } from "@playwright/test";

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  let bootstrapReads = 0;
  let revalidationStatus = 200;
  let csrf = "A".repeat(43);
  await page.route("**/api/**", async route => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/session/bootstrap") {
      bootstrapReads++;
      if (bootstrapReads > 1) await new Promise(resolve => setTimeout(resolve, 500));
      return route.fulfill({ status: revalidationStatus, contentType: "application/json", body: JSON.stringify({ data: { csrf_token: csrf }, meta: { trace_id: "fixture" } }) });
    }
    if (path === "/api/session/me") return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: {
      account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1,
      capabilities: ["platform.integration.read"], required_actions: [], session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now() + 900_000).toISOString(), absolute_expires_at: new Date(Date.now() + 3600_000).toISOString() },
    }, meta: { trace_id: "fixture" } }) });
    return route.fulfill({ status: 503, contentType: "application/json", body: "{}" });
  });
  await page.goto((process.env.BIDORA_TEST_WEB_ORIGIN || "http://127.0.0.1:3000") + "/account/security");
  const email = page.getByLabel("Email address", { exact: true });
  await email.fill("unsaved@example.test");
  const before = bootstrapReads;
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(150);
  assert.equal(await email.inputValue(), "unsaved@example.test", "focus must not unmount the active form");
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(1000);
  assert.equal(bootstrapReads, before + 1, "overlapping focus checks must coalesce");
  assert.equal(await email.inputValue(), "unsaved@example.test", "successful revalidation must retain unsaved form state");
  csrf = "B".repeat(43);
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  await page.waitForFunction(() => document.querySelector('#password-change-email')?.value === "");
  assert.equal(await email.inputValue(), "", "rotated sessions must clear previous private form state");
  revalidationStatus = 401;
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  await page.waitForURL("**/login");
  assert.equal(await page.getByRole("heading", { name: "Account security", exact: true }).count(), 0);
  console.log("PASS: background tab revalidation preserves mounted forms, coalesces requests and redirects revoked sessions (intercepted APIs only)");
} finally {
  await browser.close();
}
