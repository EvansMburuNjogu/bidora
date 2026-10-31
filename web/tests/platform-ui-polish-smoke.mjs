import test from "node:test";
import assert from "node:assert/strict";
import { chromium, expect } from "@playwright/test";

// Synthetic session and failed-section responses only; no live business service is contacted.
const origin = process.env.BIDORA_WEB_URL ?? "http://127.0.0.1:3000";
const viewports = [["desktop", { width: 1280, height: 850 }], ["mobile", { width: 390, height: 844 }]];
async function open(viewport) {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  await page.route("**/api/session/bootstrap", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { csrf_token: "A".repeat(43) }, meta: { trace_id: "synthetic" } }) }));
  await page.route("**/api/session/me", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1, capabilities: ["platform.invoice.read", "platform.payment.read", "platform.tenant.read", "platform.admin.read", "platform.integration.read", "platform.reference.read"], session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now() + 600_000).toISOString(), absolute_expires_at: new Date(Date.now() + 3_600_000).toISOString() }, required_actions: [] }, meta: { trace_id: "synthetic" } }) }));
  return { browser, page };
}
for (const [name, viewport] of viewports) {
  test(`preview table controls on ${name}`, async () => {
    const { browser, page } = await open(viewport);
    try {
      await page.goto(`${origin}/platform/plans`);
      await page.getByRole("heading", { name: "Plans & Subscriptions" }).waitFor();
      assert.equal(await page.getByRole("row", { name: /Example plan/ }).count(), 0);
      const search = page.getByRole("textbox", { name: "Search" });
      await search.fill("Example plan");
      await page.getByRole("button", { name: "Sort by Plan" }).click();
      assert.equal(await page.getByRole("columnheader", { name: /Plan/ }).first().getAttribute("aria-sort"), "ascending");
      await page.getByRole("combobox", { name: "Rows" }).selectOption("10");
      await page.getByRole("tab", { name: "Subscriptions" }).click();
      assert.equal(await search.inputValue(), "Example plan");
      await page.getByRole("button", { name: "Clear Filters" }).first().click();
      assert.equal(await search.inputValue(), "");
      await page.getByRole("button", { name: "Create Plan" }).click();
      await page.getByRole("alertdialog").getByRole("button", { name: "Save" }).isDisabled();
      await page.getByRole("alertdialog").getByRole("button", { name: "Cancel" }).click();
    } finally { await browser.close(); }
  });
  test(`overview keeps failed transactions separate on ${name}`, async () => {
    const { browser, page } = await open(viewport);
    let requests = 0;
    try {
      await page.route("**/api/platform/dashboard", (route) => { requests++; return route.fulfill({ status: 503, contentType: "application/json", body: "{}" }); });
      await page.goto(`${origin}/platform`);
      await page.getByRole("heading", { name: "Overview" }).waitFor();
      await page.getByRole("alert").getByText("Couldn’t load transactions.").waitFor();
      assert.equal(await page.getByRole("img", { name: /Illustrative/ }).count(), 0);
      await page.getByRole("button", { name: "Retry" }).click();
      await expect.poll(() => requests).toBeGreaterThan(1);
    } finally { await browser.close(); }
  });
  test(`live directory failure never substitutes examples on ${name}`, async () => {
    const { browser, page } = await open(viewport);
    try {
      await page.route("**/api/platform/tenants**", (route) => route.fulfill({ status: 503, contentType: "application/json", body: "{}" }));
      await page.goto(`${origin}/platform/tenderpreneurs`);
      await page.getByRole("alert").getByText("Couldn’t load organisations.").waitFor();
      assert.equal(await page.getByRole("button", { name: "Add Tenderpreneur" }).count(), 0);
      assert.equal(await page.getByText("Luma Works").count(), 0);
      assert.equal(await page.getByRole("region", { name: "Demo workspace" }).count(), 0);
    } finally { await browser.close(); }
  });
  test(`live administrator failure never substitutes examples on ${name}`, async () => {
    const { browser, page } = await open(viewport);
    try {
      await page.route("**/api/platform/administrators**", (route) => route.fulfill({ status: 503, contentType: "application/json", body: "{}" }));
      await page.goto(`${origin}/platform/users`);
      await page.getByRole("button", { name: "Retry" }).waitFor();
      assert.equal(await page.getByRole("button", { name: "Invite User" }).count(), 0);
      assert.equal(await page.getByText("Luma Administrator").count(), 0);
      assert.equal(await page.getByRole("region", { name: "Demo workspace" }).count(), 0);
    } finally { await browser.close(); }
  });
  test(`new secret visibility and reset on ${name}`, async () => {
    const { browser, page } = await open(viewport);
    try {
      await page.goto(`${origin}/platform/settings/sso`);
      await page.getByRole("button", { name: "Replace Secret" }).click();
      const secret = page.locator("#google-client-secret");
      await secret.fill("new-secret-example");
      assert.equal(await secret.getAttribute("type"), "password");
      await page.getByRole("button", { name: "Show password" }).click();
      assert.equal(await secret.getAttribute("type"), "text");
      assert.equal(await secret.inputValue(), "new-secret-example");
      await page.getByRole("button", { name: "Hide password" }).click();
      await page.getByRole("button", { name: "Cancel" }).click();
      assert.equal(await secret.inputValue(), "");
      assert.equal(await secret.isDisabled(), true);
    } finally { await browser.close(); }
  });
  test(`metric icons and primary actions on ${name}`, async () => {
    const { browser, page } = await open(viewport);
    try {
      for (const [path, labels] of [
        ["/platform", ["Total Tenderpreneurs", "Active Subscriptions", "Outstanding Payments", "Open Support Tickets"]],
        ["/platform/billing", ["Total Invoiced", "Paid", "Outstanding", "Failed Payments"]],
        ["/platform/ai-usage", ["Total Usage", "Estimated Cost", "Active Organisations"]],
        ["/platform/communications", ["Sent", "Delivered", "Failed", "Pending"]],
        ["/platform/support", ["Open Tickets", "Overdue Tickets"]],
      ]) {
        await page.goto(`${origin}${path}`);
        for (const label of labels) {
          const card = page.locator("div.rounded-lg.border.bg-card").filter({ has: page.getByText(label, { exact: true }) }).first();
          await expect(card.locator("svg")).toHaveCount(1);
          await expect(card.locator('p[aria-label$=": not available"]')).toHaveCount(1);
        }
      }
      await page.goto(`${origin}/platform/plans`);
      const plan = page.getByRole("button", { name: "Create Plan" });
      const brand = await plan.evaluate((el) => getComputedStyle(el).backgroundColor);
      await expect(plan.locator("svg")).toHaveCount(1);
      await page.goto(`${origin}/platform/settings/countries`);
      const country = page.getByRole("button", { name: "Add Country" }).filter({ has: page.locator("svg") }).first();
      await expect(country).toBeVisible();
      assert.equal(await country.evaluate((el) => getComputedStyle(el).backgroundColor), brand);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    } finally { await browser.close(); }
  });
  test(`shared short selectors retain values on ${name}`, async () => {
    const { browser, page } = await open(viewport);
    try {
      await page.goto(`${origin}/platform/plans`);
      await page.getByRole("button", { name: "Create Plan" }).click();
      const dialog = page.getByRole("alertdialog");
      const period = dialog.getByRole("combobox", { name: "Billing period" });
      await period.selectOption("Annual");
      await dialog.getByRole("textbox", { name: "Plan name" }).fill("Example");
      assert.equal(await period.inputValue(), "Annual");
      await expect(period.locator("..").locator("svg")).toHaveCount(1);
      await dialog.getByRole("button", { name: "Cancel" }).click();
      await page.goto(`${origin}/platform/settings/countries`);
      const status = page.getByRole("combobox", { name: "Status" });
      await status.selectOption("enabled");
      await page.getByRole("textbox", { name: "Search countries" }).fill("Uganda");
      assert.equal(await status.inputValue(), "enabled");
      await expect(status.locator("..").locator("svg")).toHaveCount(1);
    } finally { await browser.close(); }
  });
}
