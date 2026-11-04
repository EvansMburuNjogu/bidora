import assert from "node:assert/strict";
import { chromium } from "@playwright/test";
const browser = await chromium.launch({ headless: true });
try {
 const page = await browser.newPage();
 let status = 429;
 let requests = 0;
 await page.route("**/api/**", route => {
  if (new URL(route.request().url()).pathname === "/api/auth/password-recovery") {
   requests++;
   return route.fulfill({ status, headers: { "Retry-After": "900" } });
  }
  return route.fulfill({ status: 503, contentType: "application/json", body: "{}" });
 });
 const origin=process.env.BIDORA_TEST_WEB_ORIGIN || "http://127.0.0.1:3000";
 await page.goto(origin+"/forgot-password");
 await page.getByLabel("Email address",{exact:true}).fill("example@example.test");
 await page.getByRole("button",{name:"Send recovery link",exact:true}).click();
 await page.getByRole("button",{name:/Try again in/}).waitFor();
 assert.equal(await page.getByRole("button",{name:/Try again in/}).isDisabled(),true);
 assert.equal(requests,1);
 assert.equal(await page.getByLabel("New password",{exact:true}).count(),0);
 status=202;
 await page.reload();
 await page.getByLabel("Email address",{exact:true}).fill("example@example.test");
 await page.getByRole("button",{name:"Send recovery link",exact:true}).click();
 await page.waitForURL("**/forgot-password/check-email");
 assert.equal(await page.getByRole("heading",{name:"Check your email",exact:true}).count(),1);
 assert.equal(await page.locator('input[type="password"]').count(),0);
 assert.equal(await page.locator('input[name="token"]').count(),0);
 await page.goto(origin+"/reset-password");
 await page.getByLabel("Recovery code",{exact:true}).waitFor();
 await page.getByLabel("New password",{exact:true}).waitFor();
 console.log("PASS: recovery respects rate limits and separates email confirmation from code/password reset (intercepted APIs only)");
} finally { await browser.close(); }
