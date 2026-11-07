import assert from "node:assert/strict";
import { chromium } from "@playwright/test";
const browser = await chromium.launch({ headless: true });
try {
 const page = await browser.newPage();
 let status = 429;
 let requests = 0;
 let resetStatus = 400;
 const resets = [];
 await page.route("**/api/**", route => {
  if (new URL(route.request().url()).pathname === "/api/auth/password-policy") return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{minimum_password_bytes:12,maximum_password_bytes:128},meta:{trace_id:"synthetic"}})});
  if (new URL(route.request().url()).pathname === "/api/auth/password-recovery") {
   requests++;
   return route.fulfill({ status, headers: { "Retry-After": "900" } });
  }
  if (new URL(route.request().url()).pathname === "/api/auth/password-reset") {
   resets.push(route.request().postDataJSON());
   return route.fulfill({ status: resetStatus });
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
 await page.locator('a[href="/reset-password"]').click();
 await page.waitForURL("**/reset-password");
 const token = "a".repeat(42) + "A";
 await page.getByLabel("Recovery code",{exact:true}).fill(token);
 await page.getByLabel("New password",{exact:true}).fill("A new secure passphrase");
 await page.getByLabel("Confirm new password",{exact:true}).fill("A different passphrase");
 await page.getByRole("button",{name:"Reset password",exact:true}).click();
 await page.getByText("Passwords do not match. Enter the same password again.").waitFor();
 assert.equal(resets.length,0);
 await page.getByLabel("Confirm new password",{exact:true}).fill("A new secure passphrase");
 await page.getByRole("button",{name:"Reset password",exact:true}).click();
 await page.getByText(/The recovery code or password requirements are invalid/).waitFor();
 assert.deepEqual(resets,[{token,new_password:"A new secure passphrase"}]);
 resetStatus=204;
 await page.getByRole("button",{name:"Reset password",exact:true}).click();
 await page.waitForURL("**/login");
 assert.equal(resets.length,2);
 console.log("PASS: recovery rate limits, reset navigation, password confirmation, invalid code and reset success (intercepted APIs only)");
} finally { await browser.close(); }
