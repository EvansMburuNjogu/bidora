import assert from "node:assert/strict";
import test from "node:test";
import {chromium,expect} from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://localhost:3194";
if(!/^http:\/\/(localhost|127\.0\.0\.1):\d+$/.test(origin))throw Error("Loopback only; APIs are intercepted.");
const token="A".repeat(43),trace="11111111-1111-4111-8111-111111111111";
async function fixture(existing=false){
 const browser=await chromium.launch(),page=await browser.newPage();const state={writes:[],authenticated:false,accepted:false};
 await page.route("**/api/**",async route=>{
  const r=route.request(),p=new URL(r.url()).pathname;let status=200,headers={},body;
  if(p==="/api/auth/password-policy")body={data:{minimum_password_bytes:20,maximum_password_bytes:128}};
  else if(p==="/api/auth/administrator-invitation/preview")body={data:{email:"amina@example.test",first_name:"Amina",last_name:"Njoroge",role_key:"PLATFORM_OWNER",expires_at:new Date(Date.now()+3600000).toISOString(),account_exists:existing}};
  else if(p==="/api/auth/administrator-invitation/setup"||p==="/api/auth/login"){state.writes.push({p,body:r.postDataJSON()});status=202;body={data:{mfa_required:true,expires_at:new Date(Date.now()+300000).toISOString()},meta:{trace_id:trace}};headers={"X-Bidora-Challenge-Expires-At":body.data.expires_at};}
  else if(p==="/api/auth/login-code"){state.writes.push({p,body:r.postDataJSON()});if(r.postDataJSON().code!=="123456"){status=400;body={error:{code:"MFA_INVALID",message:"Invalid code."}}}else{status=204;state.authenticated=true;if(existing&&!state.accepted)headers={"X-Bidora-Invitation-Pending":"1"};}}
  else if(p==="/api/auth/administrator-invitation/accept"){state.accepted=true;state.authenticated=false;state.writes.push({p,body:r.postDataJSON()});status=204;}
  else if(p==="/api/session/bootstrap"){status=state.authenticated?200:401;body=state.authenticated?{data:{csrf_token:"C".repeat(43)},meta:{trace_id:trace}}:{error:{code:"SESSION_INVALID"}};}
  else if(p==="/api/session/me")body={data:{account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:["platform.admin.read"],required_actions:[],session:{assurance_level:"AAL2",idle_expires_at:new Date(Date.now()+3600000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()}},meta:{trace_id:trace}};
  else if(p==="/api/session/availability")body={data:{maintenance_active:false,maintenance_message:""},meta:{trace_id:trace}};
  else{status=503;body={error:{code:"UNAVAILABLE"}}}
  await route.fulfill({status,headers,contentType:status===204?undefined:"application/json",body:status===204?undefined:JSON.stringify(body)});
 });return{browser,page,state};
}
test("Invitation fragment is cleared; password setup requires email MFA before the Platform Portal",{timeout:60000},async()=>{
 const {browser,page,state}=await fixture();try{
  await page.goto(origin+"/administrator-invitation#code="+token);
  await expect(page.getByText("amina@example.test",{exact:true})).toBeVisible();assert.equal(new URL(page.url()).hash,"");assert.equal(new URL(page.url()).search,"");
  await expect(page.getByLabel("First name",{exact:true})).toHaveValue("Amina");await expect(page.getByLabel("First name",{exact:true})).toHaveAttribute("readonly","");
  await page.getByLabel("Create password",{exact:true}).fill("Synthetic-password-123456");await page.getByLabel("Confirm password",{exact:true}).fill("mismatch");await page.getByRole("button",{name:"Set password and continue",exact:true}).click();await expect(page.getByText("Passwords do not match.",{exact:true})).toBeVisible();assert.equal(state.writes.length,0);
  await page.getByLabel("Confirm password",{exact:true}).fill("Synthetic-password-123456");await page.getByRole("button",{name:"Set password and continue",exact:true}).click();await expect(page).toHaveURL(/\/auth\/login-code$/);assert.equal(state.authenticated,false);assert.equal(state.writes[0].p,"/api/auth/administrator-invitation/setup");assert.equal("token" in state.writes[0].body,false);
  const code=page.locator('input[autocomplete="one-time-code"]');await code.fill("123456");await page.getByRole("button",{name:/Verify|Continue|Sign in/}).click();await expect(page).toHaveURL(/\/platform$/);assert.equal(state.authenticated,true);
 }finally{await browser.close()}
});
test("Existing accounts keep their password and accept only after authenticated email verification",{timeout:60000},async()=>{
 const {browser,page,state}=await fixture(true);try{
  await page.goto(origin+"/administrator-invitation#code="+token);await expect(page.getByLabel("Password",{exact:true})).toBeVisible();await expect(page.getByLabel("Create password",{exact:true})).toHaveCount(0);
  await page.getByLabel("Password",{exact:true}).fill("Existing-password-123456");await page.getByRole("button",{name:"Sign in",exact:true}).click();await expect(page).toHaveURL(/\/auth\/login-code$/);
  await page.locator('input[autocomplete="one-time-code"]').fill("123456");await page.getByRole("button",{name:/Verify|Continue|Sign in/}).click();await expect(page).toHaveURL(/\/administrator-invitation$/);await page.getByRole("button",{name:"Accept invitation",exact:true}).click();await expect(page.getByText("Invitation accepted. Sign in again to verify your new administrator access.")).toBeVisible();assert.equal(state.accepted,true);assert.equal(state.authenticated,false);assert.equal(state.writes.some(r=>r.p.endsWith("/setup")),false);
 }finally{await browser.close()}
});
