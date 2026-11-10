import assert from "node:assert/strict";
import test from "node:test";
import {chromium,expect} from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3000";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use loopback; APIs intercepted.");
test("Older verification emails remain usable by pasting the original full code",{timeout:30000},async()=>{
 const browser=await chromium.launch({headless:true});try{
  const page=await browser.newPage();const legacy="A".repeat(43);let body=null;
  await page.route("**/api/**",async route=>{
   if(new URL(route.request().url()).pathname==="/api/auth/verify-email"){body=route.request().postDataJSON();return route.fulfill({status:204,body:""});}
   return route.fulfill({status:401,contentType:"application/json",body:"{}"});
  });
  await page.goto(origin+"/verify-email");
  await page.getByRole("textbox",{name:"Digit 1 of 6",exact:true}).evaluate((node,value)=>{
   const data=new DataTransfer();data.setData("text/plain",value);node.dispatchEvent(new ClipboardEvent("paste",{bubbles:true,cancelable:true,clipboardData:data}));
  },legacy);
  await page.getByRole("button",{name:"Verify email",exact:true}).click();
  await expect(page).toHaveURL(origin+"/login");assert.deepEqual(body,{token:legacy});
 }finally{await browser.close()}
});
