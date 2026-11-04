import assert from "node:assert/strict";
import { chromium } from "@playwright/test";

const browser=await chromium.launch({headless:true});
try {
 const page=await browser.newPage();
 let verified=false;
 await page.route("**/api/**",route=>{
  const path=new URL(route.request().url()).pathname;
  if(path==="/api/session/bootstrap")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{csrf_token:"A".repeat(43)},meta:{trace_id:"fixture"}})});
  if(path==="/api/session/me")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{account_type:"PLATFORM_ADMIN",portal:verified?{key:"PLATFORM_ADMIN",path:"/platform"}:null,platform_context_generation:1,capabilities:["platform.integration.read"],required_actions:verified?[]:["COMPLETE_PLATFORM_STEP_UP"],session:{assurance_level:"AAL2",idle_expires_at:new Date(Date.now()+900000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()}},meta:{trace_id:"fixture"}})});
  if(path==="/api/auth/logout")return route.fulfill({status:204});
  if(path==="/api/auth/login")return route.fulfill({status:202,contentType:"application/json",body:JSON.stringify({data:{mfa_required:true,expires_at:new Date(Date.now()+300000).toISOString()}})});
  if(path==="/api/auth/login-code"){verified=true;return route.fulfill({status:204});}
  return route.fulfill({status:503,contentType:"application/json",body:"{}"});
 });
 const origin=process.env.BIDORA_TEST_WEB_ORIGIN||"http://127.0.0.1:3000";
 await page.goto(origin+"/auth/resolve");
 await page.getByRole("heading",{name:"Verify your email",exact:true}).waitFor();
 assert.equal(await page.getByRole("button",{name:"Verify with authenticator",exact:true}).count(),0);
 await page.getByRole("button",{name:"Sign in again",exact:true}).click();
 await page.waitForURL("**/login");
 await page.getByLabel("Work email",{exact:true}).fill("member@example.test");
 await page.getByLabel("Password",{exact:true}).fill("synthetic password");
 await page.getByRole("button",{name:"Sign in",exact:true}).click();
 await page.waitForURL("**/auth/login-code");
 for(let i=1;i<=6;i++)await page.getByLabel(`Digit ${i} of 6`,{exact:true}).fill(String(i));
 await page.getByRole("button",{name:"Verify & Continue",exact:true}).click();
 await page.waitForURL("**/platform");
 await page.getByRole("heading",{name:"Overview",exact:true}).waitFor();
 await page.goto(origin+"/account/security/authenticator");
 await page.waitForURL("**/account/security");
 assert.equal(await page.getByRole("button",{name:"Set up authenticator",exact:true}).count(),0);
 await page.getByText("Sign-in verification uses a one-time code sent to your email. Codes expire after five minutes.",{exact:true}).waitFor();
 console.log("PASS: stale Platform verification → fresh email-code sign-in → Platform, with retired authenticator bookmark redirected (intercepted APIs only)");
} finally {await browser.close();}
