import { spawn } from "node:child_process";
import { once } from "node:events";
import { rm } from "node:fs/promises";
import path from "node:path";
import { chromium } from "@playwright/test";

// Invoked only by the isolated Go integration test. Never print cookies,
// request bodies, server logs or browser traces; all tokens stay in memory.
const web = new URL(process.env.PA05_WEB_ORIGIN);
const api = new URL(process.env.PA05_API_ORIGIN);
if (web.protocol !== "http:" || api.protocol !== "http:" || web.hostname !== "127.0.0.1" || api.hostname !== "127.0.0.1" || !/^[0-9a-f-]{36}$/i.test(process.env.PA05_TARGET_ID ?? "")) throw new Error("isolated browser configuration invalid");
const dist = `.next-auth-test-${process.pid}`;
const next = spawn(process.execPath, [path.join(process.cwd(), "node_modules/next/dist/bin/next"), "dev", "-H", "127.0.0.1", "-p", web.port], {
  cwd: process.cwd(),
  env: { PATH: process.env.PATH, HOME: process.env.HOME, NODE_ENV: "development", NEXT_TELEMETRY_DISABLED: "1", BIDORA_API_ORIGIN: api.origin, BIDORA_WEB_ORIGIN: web.origin, BIDORA_BFF_CLIENT_IP_MODE: "direct-peer", BIDORA_NEXT_TEST_DIST_DIR: dist },
  stdio: ["ignore", "pipe", "pipe"],
});
next.stdout.resume(); next.stderr.resume();
let browser;
let stage = "start";
const statuses = { bootstrap: 0, me: 0, detail: 0 };
async function cookies(context, who) {
  const session = process.env[`PA05_${who}_SESSION`];
  const csrf = process.env[`PA05_${who}_CSRF`];
  if (!/^[A-Za-z0-9_-]{43}$/.test(session ?? "") || !/^[A-Za-z0-9_-]{43}$/.test(csrf ?? "")) throw new Error("cookie shape invalid");
  await context.addCookies([
    { name: "bidora_dev_session", value: session, url: web.origin, httpOnly: true, sameSite: "Lax" },
    { name: "bidora_dev_csrf", value: csrf, url: web.origin, httpOnly: false, sameSite: "Lax" },
  ]);
}
try {
  stage = "web-ready";
  let ready = false;
  for (let i = 0; i < 160; i++) {
    try { const r = await fetch(web.origin + "/api/session/bootstrap"); if (r.status === 401) { ready = true; break; } } catch {}
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  if (!ready) throw new Error("isolated web not ready");
  browser = await chromium.launch({ headless: true });
  stage = "maker-detail";
  const maker = await browser.newContext();
  await cookies(maker, "MAKER");
  const page = await maker.newPage();
  page.on("response", (response) => {
    const route = new URL(response.url()).pathname;
    if (route === "/api/session/bootstrap") statuses.bootstrap = response.status();
    if (route === "/api/session/me") statuses.me = response.status();
    if (route.startsWith("/api/platform/administrators/")) statuses.detail = response.status();
  });
  stage = "maker-navigation";
  await page.goto(web.origin + `/platform/administrators/${process.env.PA05_TARGET_ID}`);
  stage = "maker-heading";
  await page.getByRole("heading", { name: "Administrator details" }).waitFor({ timeout: 15000 });
  stage = "maker-account";
  await page.getByRole("heading", { name: "Account access" }).waitFor({ timeout: 15000 });
  stage = "maker-disable";
  await page.getByRole("button", { name: "Disable administrator" }).click();
  const dialog = page.getByRole("alertdialog");
  await dialog.getByRole("textbox", { name: "Reason" }).fill("Browser synthetic owner disable");
  await dialog.getByRole("button", { name: "Disable administrator" }).click();
  await page.getByText("Your request has been submitted. The account is awaiting approval.").waitFor({ timeout: 12000 });
  stage = "maker-denied";
  await page.goto(web.origin + "/platform/approvals");
  const makerItem = page.locator("li").filter({ hasText: "Browser synthetic owner disable" });
  await makerItem.getByRole("textbox", { name: "Decision reason" }).fill("Self approval denied");
  await makerItem.getByRole("button", { name: "Approve" }).click();
  await page.getByText("You cannot decide this change.", { exact: false }).waitFor({ timeout: 12000 });
  stage = "checker-approve";
  const checker = await browser.newContext();
  await cookies(checker, "CHECKER");
  const review = await checker.newPage();
  await review.goto(web.origin + "/platform/approvals");
  const item = review.locator("li").filter({ hasText: "Browser synthetic owner disable" });
  await item.getByRole("textbox", { name: "Decision reason" }).fill("Independent browser checker");
  await item.getByRole("button", { name: "Approve" }).click();
  await review.getByText("No pending administrator changes.").waitFor({ timeout: 12000 });
  console.log("isolated_browser_maker_checker_passed");
} catch {
  console.error(`isolated_browser_stage_failed:${stage} bootstrap=${statuses.bootstrap} me=${statuses.me} detail=${statuses.detail}`);
  process.exitCode = 1;
} finally {
  if (browser) await browser.close();
  if (next.exitCode === null) {
    next.kill("SIGTERM");
    await Promise.race([once(next, "exit"), new Promise((resolve) => setTimeout(resolve, 5000))]);
    if (next.exitCode === null) next.kill("SIGKILL");
  }
  await rm(path.join(process.cwd(), dist), { recursive: true, force: true });
}
