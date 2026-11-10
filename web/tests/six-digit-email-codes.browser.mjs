import assert from "node:assert/strict";
import test from "node:test";
import {chromium,expect} from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3000";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use loopback; APIs are intercepted.");
const reference="00000000-0000-4000-8000-000000000001";
for(const mode of ["verify-email","reset-password"])test(`${mode} uses six numeric inputs and submits reference with leading zero code`,{timeout:30000},async()=>{
 const browser=await chromium.launch({headless:true});try{
  const page=await browser.newPage();const writes=[];
  await page.route("**/api/**",async route=>{
   const req=route.request(),path=new URL(req.url()).pathname;
   if(path==="/api/auth/password-policy")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{minimum_password_bytes:12,maximum_password_bytes:128}})});
   if(path===`/api/auth/${mode==="verify-email"?"verify-email":"password-reset"}`){writes.push(req.postDataJSON());return route.fulfill({status:204,body:""});}
   return route.fulfill({status:401,contentType:"application/json",body:"{}"});
  });
  await page.goto(`${origin}/${mode}?challenge_ref=${reference}`);
  await expect(page.getByRole("group",{name:"Six-digit verification code",exact:true})).toBeVisible();
  await expect(page.getByRole("textbox",{name:/Digit \d of 6/})).toHaveCount(6);
  await expect(page.getByText("Need a new code?",{exact:true})).toHaveCount(0);
  if(mode==="verify-email")await expect(page.getByLabel("Email address",{exact:true})).toHaveCount(0);
  await page.getByRole("textbox",{name:"Digit 1 of 6",exact:true}).fill("001234");
  if(mode==="reset-password"){
   await page.getByLabel("New password",{exact:true}).fill("synthetic replacement password");
   await page.getByLabel("Confirm new password",{exact:true}).fill("synthetic replacement password");
  }
  await page.getByRole("button",{name:mode==="verify-email"?"Verify email":"Reset password",exact:true}).click();
  await expect(page).toHaveURL(origin+"/login");
  assert.equal(writes.length,1);assert.equal(writes[0].token,"001234");assert.equal(writes[0].challenge_ref,reference);assert.equal(writes[0].confirm_password,undefined);
 }finally{await browser.close()}
});
