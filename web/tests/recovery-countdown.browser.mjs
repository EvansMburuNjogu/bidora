import assert from "node:assert/strict";
import test from "node:test";
import {chromium,expect} from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3000";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use loopback; all APIs are intercepted.");
test("Recovery cooldown expires visibly and permits one explicit retry",{timeout:30000},async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage();let requests=0;
  await page.route("**/api/**",async route=>{
   if(new URL(route.request().url()).pathname==="/api/auth/password-recovery"){
    requests++;
    return route.fulfill(requests===1?{status:429,headers:{"Retry-After":"2"},contentType:"application/json",body:"{}"}:{status:202,body:""});
   }
   return route.fulfill({status:401,contentType:"application/json",body:"{}"});
  });
  await page.goto(origin+"/forgot-password");
  await page.getByLabel("Email address",{exact:true}).fill("synthetic@example.test");
  await page.getByRole("button",{name:"Send recovery link",exact:true}).click();
  const warning=page.getByText("Too many recovery requests. Wait before requesting another link.",{exact:true});
  await expect(warning).toBeVisible();
  await expect(page.getByRole("button",{name:/Try again in/})).toBeDisabled();
  await expect(page.getByRole("button",{name:"Send recovery link",exact:true})).toBeEnabled({timeout:6000});
  await expect(warning).toBeHidden();assert.equal(requests,1);
  await page.getByRole("button",{name:"Send recovery link",exact:true}).click();
  await expect(page).toHaveURL(origin+"/forgot-password/check-email");assert.equal(requests,2);
 }finally{await browser.close()}
});
