import test from "node:test";
import assert from "node:assert/strict";
import { chromium } from "@playwright/test";

// Synthetic frontend-only smoke: intercept canonical session reads; never call a live backend.
const origin = process.env.BIDORA_WEB_URL ?? "http://127.0.0.1:3000";
async function openForm(viewport) {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  await page.route("**/api/session/bootstrap", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { csrf_token: "A".repeat(43) }, meta: { trace_id: "synthetic" } }) }));
  await page.route("**/api/session/me", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { account_type: "PLATFORM_ADMIN", portal: { key: "PLATFORM_ADMIN", path: "/platform" }, platform_context_generation: 1, capabilities: ["platform.reference.read"], session: { assurance_level: "AAL2", idle_expires_at: new Date(Date.now() + 600_000).toISOString(), absolute_expires_at: new Date(Date.now() + 3_600_000).toISOString() }, required_actions: [] }, meta: { trace_id: "synthetic" } }) }));
  await page.goto(`${origin}/platform/settings/countries`);
  await page.getByRole("button", { name: "Add Country" }).first().click();
  const dialog = page.getByRole("alertdialog");
  await dialog.waitFor();
  return { browser, page, dialog };
}
async function select(dialog, label, term, option) {
  const input = dialog.getByRole("combobox", { name: label });
  await input.click();
  await input.fill(term);
  await dialog.getByRole("option", { name: option, exact: true }).click();
  await input.waitFor();
  assert.equal(await input.inputValue(), option);
  return input;
}
for (const [name, viewport] of [["desktop", { width: 1280, height: 850 }], ["mobile", { width: 390, height: 844 }]]) {
  test(`country dropdown selection on ${name}`, async () => {
    const { browser, page, dialog } = await openForm(viewport);
    try {
      const country = await select(dialog, "Country", "Kenya", "Kenya");
      const currency = dialog.getByRole("combobox", { name: "Default Currency" });
      const language = dialog.getByRole("combobox", { name: "Default Language" });
      const zone = dialog.getByRole("combobox", { name: "Default Time Zone" });
      assert.match(await currency.inputValue(), /KES/);
      assert.equal(await language.inputValue(), "English");
      assert.equal(await zone.inputValue(), "Africa/Nairobi");
      await country.click(); await country.pressSequentially("Uga");
      assert.equal(await country.inputValue(), "Uga");
      await country.press("Escape");
      assert.equal(await country.inputValue(), "Kenya");
      await select(dialog, "Default Language", "Swahili", "Swahili");
      assert.match(await currency.inputValue(), /KES/);
      assert.equal(await zone.inputValue(), "Africa/Nairobi");
      await language.click(); await language.press("Escape");
      assert.equal(await language.inputValue(), "Swahili");
      await dialog.getByRole("button", { name: "Add Country" }).click();
      await dialog.getByText("Country already listed.").waitFor();
      await select(dialog, "Country", "Uganda", "Uganda");
      assert.match(await currency.inputValue(), /UGX/);
      assert.equal(await language.inputValue(), "English");
      assert.equal(await zone.inputValue(), "Africa/Kampala");
      await select(dialog, "Default Currency", "US Dollar", "US Dollar (USD)");
      await currency.click();
      assert.equal(await dialog.getByRole("option", { name: "US Dollar (USD)" }).getAttribute("aria-selected"), "true");
      await currency.press("Escape");
      await select(dialog, "Default Language", "Swahili", "Swahili");
      assert.match(await currency.inputValue(), /USD/);
      assert.equal(await zone.inputValue(), "Africa/Kampala");
      await zone.click(); await zone.fill("Africa/Nairo"); await zone.press("End"); await zone.press("Enter");
      assert.equal(await zone.inputValue(), "Africa/Nairobi");
      assert.match(await currency.inputValue(), /USD/);
      assert.equal(await language.inputValue(), "Swahili");
      await zone.click(); await zone.press("Escape");
      assert.equal(await zone.inputValue(), "Africa/Nairobi");
      await dialog.getByRole("button", { name: "Add Country" }).click();
      await page.getByRole("row", { name: /Uganda.*USD.*Swahili.*Africa\/Nairobi/ }).waitFor();
      const status = page.getByRole("combobox", { name: "Status" });
      await status.selectOption("enabled");
      assert.equal(await status.inputValue(), "enabled");
      await page.getByRole("textbox", { name: "Search countries" }).fill("Ug");
      assert.equal(await status.inputValue(), "enabled");
      await page.getByRole("row", { name: /Uganda.*USD/ }).waitFor();
      await page.getByRole("button", { name: "Actions for Uganda" }).click();
      await page.getByRole("menuitem", { name: "Edit" }).click();
      await dialog.waitFor();
      assert.match(await dialog.getByRole("combobox", { name: "Default Currency" }).inputValue(), /USD/);
      await dialog.getByRole("button", { name: "Cancel" }).click();
    } finally { await browser.close(); }
  });
}
