import assert from "node:assert/strict";
import { chromium, expect } from "@playwright/test";

const origin = process.env.BIDORA_WEB_URL ?? "http://127.0.0.1:3035";
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw Error("Use a loopback origin.");
const id = "11111111-1111-4111-8111-111111111111", at = "2026-01-01T12:00:00Z";
const envelope = data => ({ data, meta: { trace_id: id } });
const saved = {
 id, setting_kind: "SMTP", sequence: 1, version: 1, state: "ACTIVE", test_status: "PASSED", test_code: "SMTP_STARTTLS_VERIFIED",
 created_at: at, tested_at: at, activated_at: at, superseded_at: null,
 configuration: { setting_kind: "SMTP", host: "smtp.saved.example", port: 2525, credential_reference_configured: true, trust_roots_reference_configured: true, starttls_required: true,
 sender_name: "Saved sender", sender_address: "saved@example.test", reply_to_address: null,
 connect_timeout_seconds: 10, command_timeout_seconds: 15, delivery_timeout_seconds: 30, max_concurrency: 4 },
};
const capabilities = ["platform.admin.read", "platform.integration.read", "platform.integration.smtp.manage", "platform.integration.smtp.activate", "platform.integration.smtp.test"];
const browser = await chromium.launch({ headless: true });
try {
 for (const historyMode of ["slow", "failed"]) {
  const page = await browser.newPage();
  let historyDone = false, activeReads = 0, writes = 0;
  await page.route("**/api/**", async route => {
   const p = new URL(route.request().url()).pathname;
   if (route.request().method() !== "GET") { writes++; return route.fulfill({ status: 503 }); }
   let data;
   if (p === "/api/session/bootstrap") data = envelope({ csrf_token: "A".repeat(43) });
   else if (p === "/api/session/me") data = envelope({ account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1, capabilities, session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now()+600000).toISOString(), absolute_expires_at: new Date(Date.now()+3600000).toISOString() }, required_actions: [] });
   else if (p === "/api/session/availability") data = envelope({ maintenance_active: false, maintenance_message: "" });
   else if (p === "/api/platform/smtp") { activeReads++; data = envelope(saved); }
   else if (p === "/api/platform/smtp/versions") {
    if (historyMode === "failed") return route.fulfill({ status: 503, contentType: "application/json", body: "{}" });
    await new Promise(resolve => setTimeout(resolve, 2500)); historyDone = true; data = envelope([saved]);
   } else return route.fulfill({ status: 503, contentType: "application/json", body: "{}" });
   return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(data) });
  });
  await page.goto(origin + "/platform/settings/smtp");
  await expect(page.getByLabel("Server", { exact: true })).toHaveValue("smtp.saved.example");
  await expect(page.getByLabel("Port", { exact: true })).toHaveValue("2525");
  await expect(page.getByLabel("Sender Name", { exact: true })).toHaveValue("Saved sender");
  await expect(page.getByLabel("Sender Email", { exact: true })).toHaveValue("saved@example.test");
  await expect(page.getByLabel("Password", { exact: true })).toHaveValue("");
  await expect(page.getByText("Default SMTP active", { exact: true })).toBeVisible();
  assert.ok(activeReads >= 1); assert.equal(writes, 0);
  if (historyMode === "slow") assert.equal(historyDone, false, "history delayed active form");
  await page.getByLabel("Server", { exact: true }).fill("smtp.edited.example");
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByLabel("Server", { exact: true })).toHaveValue("smtp.saved.example");
  if (historyMode === "failed") await expect(page.locator('p[role="alert"]')).toContainText("Previous settings could not be loaded");
  await page.close();
 }
 console.log("PASS: current SMTP inputs autofill on load despite slow/failed history; no secret exposure or writes; cancel restores saved fields");
} finally { await browser.close(); }
