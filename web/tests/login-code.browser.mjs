import assert from "node:assert/strict";
import { chromium } from "@playwright/test";

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.route("**/api/**", async route => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/auth/login") return route.fulfill({ status: 202, contentType: "application/json", body: JSON.stringify({ data: { mfa_required: true, expires_at: new Date(Date.now() + 65_000).toISOString() } }) });
    if (path === "/api/auth/login-code-resend") return route.fulfill({ status: 202, headers: { "X-Bidora-Challenge-Expires-At": new Date(Date.now() + 300_000).toISOString() } });
    return route.fulfill({ status: 503, contentType: "application/json", body: "{}" });
  });
  await page.goto(process.env.BIDORA_TEST_WEB_ORIGIN || "http://127.0.0.1:3000/login");
  await page.getByLabel("Work email", { exact: true }).fill("alice@example.com");
  await page.getByLabel("Password", { exact: true }).fill("synthetic password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.waitForURL("**/auth/login-code");
  await page.getByText("For a***@e***.com", { exact: true }).waitFor();
  const timer = page.getByRole("timer");
  await timer.waitFor();
  await page.waitForFunction(() => document.querySelector('[role="timer"]')?.textContent?.includes("Expires in"));
  const first = await timer.textContent();
  await page.waitForTimeout(2100);
  assert.notEqual(await timer.textContent(), first);
  assert.ok(!(await page.locator("body").innerText()).includes("alice@example.com"));
  assert.ok(!(await page.locator("body").innerText()).includes("Codes expire after five minutes"));
  await page.getByRole("button", { name: "Resend code", exact: true }).click();
  await page.getByRole("button", { name: /Resend in \d+s/ }).waitFor();
  assert.match(await timer.textContent(), /Expires in [45]:\d{2}/);
  console.log("PASS: masked email, ticking expiry and server-deadline resend reset (intercepted APIs only)");
} finally {
  await browser.close();
}
