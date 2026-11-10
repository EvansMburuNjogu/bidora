import assert from "node:assert/strict";
import test from "node:test";
import {chromium,expect} from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3000";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use loopback; all APIs intercepted.");
test("Failed invitation lookup stops loading and keeps identity fields disabled",{timeout:30000},async()=>{
 const browser=await chromium.launch({headless:true});try{
  const page=await browser.newPage();let registrations=0;
  await page.route("**/api/**",async route=>{
   const p=new URL(route.request().url()).pathname;
   if(p==="/api/auth/invitation-register")registrations++;
   if(p==="/api/auth/password-policy")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{minimum_password_bytes:12,maximum_password_bytes:128}})});
   return route.fulfill({status:p==="/api/auth/invitation-details"?503:401,contentType:"application/json",body:"{}"});
  });
  await page.goto(origin+"/invitations/platform#code="+"A".repeat(43));
  await expect(page.getByText("Unable to load this invitation. Reopen the original invitation link from your email.",{exact:true})).toBeVisible();
  for(const label of ["Email address","User type"]){await expect(page.getByLabel(label,{exact:true})).toBeDisabled();await expect(page.getByLabel(label,{exact:true})).toHaveAttribute("placeholder","Unavailable");}
  await expect(page.getByRole("button",{name:"Set password",exact:true})).toBeDisabled();assert.equal(registrations,0);
 }finally{await browser.close()}
});
