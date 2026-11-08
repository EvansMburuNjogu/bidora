import assert from "node:assert/strict";
import test from "node:test";
import { chromium, expect } from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3120";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use an isolated loopback server");
const envelope=data=>({data,meta:{trace_id:"synthetic-security"}});
const seed=()=>({idle_seconds:1800,absolute_seconds:43200,version:4,security:{failed_login_attempts:5,failure_window_seconds:900,lockout_seconds:900,minimum_password_bytes:12,password_reset_seconds:1800,allow_multiple_sessions:true,admin_idle_seconds:604800,admin_absolute_seconds:604800}});
async function open(viewport={width:1280,height:900},canSave=true){
 const browser=await chromium.launch({headless:true});const page=await browser.newPage({viewport});const state={saved:seed(),writes:[],readStatus:200,writeStatus:200};
 await page.route("**/api/**",async route=>{const req=route.request(),p=new URL(req.url()).pathname;let data,status=200;
 if(p==="/api/session/bootstrap")data=envelope({csrf_token:"A".repeat(43)});
 else if(p==="/api/session/me")data=envelope({account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:["platform.admin.read",...(canSave?["platform.governance.approve"]:[])],session:{assurance_level:"AAL2",idle_expires_at:new Date(Date.now()+600000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()},required_actions:[]});
 else if(p==="/api/session/availability")data=envelope({maintenance_active:false,maintenance_message:""});
 else if(p==="/api/session/countries")data=envelope({countries:[{country_code:"KE",name:"Kenya",currency:"KES",language:"en",timezone:"Africa/Nairobi",locale:"en-KE",enabled:true,is_default:true,version:1,updated_at:"2026-01-01T12:00:00Z"}],default_country:"KE",billing_currency:"KES",organization_registration_enabled:true});
 else if(p==="/api/platform/security/session-policy"){
  if(req.method()==="PUT"){state.writes.push({body:req.postDataJSON(),headers:req.headers()});status=state.writeStatus;if(status===200){state.saved={...state.saved,...req.postDataJSON(),version:state.saved.version+1};data=envelope(state.saved)}}
  else{status=state.readStatus;data=envelope(state.saved)}
 }else{status=503}
 await route.fulfill({status,contentType:"application/json",body:JSON.stringify(data??{error:{code:"UNAVAILABLE",message:"Unavailable",retryable:true,field_errors:[]}})});
 });return{browser,page,state};
}
test("security controls save canonical values, persist after reload and fit mobile",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:390,height:844});try{
 await page.goto(origin+"/platform/settings/security");await expect(page.getByLabel("Minimum password length in bytes")).toHaveValue("12");
 await page.getByLabel("Minimum password length in bytes").fill("20");await page.getByLabel("Maximum failed password attempts").fill("3");await page.getByLabel("Failed attempt window in minutes").fill("5");await page.getByLabel("Account lockout duration in minutes").fill("10");await page.getByLabel("Password reset expiry in minutes").fill("10");await page.getByLabel("Administrator inactivity timeout in minutes").fill("15");await page.getByLabel("Administrator maximum session duration in minutes").fill("60");await page.getByLabel("Allow multiple active sessions").uncheck();
 await page.getByRole("button",{name:"Save Changes",exact:true}).click();await page.getByRole("alertdialog").getByRole("button",{name:"Save Changes",exact:true}).click();await expect(page.getByRole("alertdialog")).toHaveCount(0);
 assert.equal(state.writes.length,1);const sent=state.writes[0];assert.equal(sent.headers["if-match"],'"4"');assert.equal(sent.body.security.minimum_password_bytes,20);assert.equal(sent.body.security.failure_window_seconds,300);assert.equal(sent.body.security.lockout_seconds,600);assert.equal(sent.body.security.password_reset_seconds,600);assert.equal(sent.body.security.allow_multiple_sessions,false);assert.equal(sent.body.security.admin_idle_seconds,900);assert.equal(sent.body.security.admin_absolute_seconds,3600);
 await page.reload();await expect(page.getByLabel("Minimum password length in bytes")).toHaveValue("20");await expect(page.getByLabel("Allow multiple active sessions")).not.toBeChecked();assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 }finally{await browser.close()}
});
test("every numeric security field stays empty when cleared and types 60 without a leading zero",{timeout:60000},async()=>{
 const {browser,page,state}=await open();try{
  await page.goto(origin+"/platform/settings/security");
  const fields=["Maximum failed password attempts","Failed attempt window in minutes","Account lockout duration in minutes","Minimum password length in bytes","Administrator inactivity timeout in minutes","Administrator maximum session duration in minutes","Password reset expiry in minutes"];
  await expect(page.getByLabel(fields[0],{exact:true})).toHaveValue("5");
  for(const label of fields){
   const field=page.getByLabel(label,{exact:true}),original=await field.inputValue();
   await field.fill("");await expect(field).toHaveValue("");
   await expect(page.getByRole("button",{name:"Save Changes",exact:true})).toBeDisabled();
   await field.pressSequentially("60");await expect(field).toHaveValue("60");
   await field.fill("00060");await expect(field).toHaveValue("60");
   await field.fill("0");await expect(field).toHaveValue("0");
   await expect(page.getByRole("button",{name:"Save Changes",exact:true})).toBeDisabled();
   await page.getByRole("button",{name:"Cancel",exact:true}).click();await expect(field).toHaveValue(original);
  }
  const length=page.getByLabel("Minimum password length in bytes");
  await length.press("ArrowUp");await expect(length).toHaveValue("13");
  assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
test("failed saves retain inputs and retry key; cancel and invalid ranges do not write",{timeout:60000},async()=>{
 const{browser,page,state}=await open();try{await page.goto(origin+"/platform/settings/security");await expect(page.getByLabel("Minimum password length in bytes")).toHaveValue("12");await page.getByLabel("Minimum password length in bytes").fill("10");await expect(page.getByRole("button",{name:"Save Changes",exact:true})).toBeDisabled();await page.getByRole("button",{name:"Cancel",exact:true}).click();await expect(page.getByLabel("Minimum password length in bytes")).toHaveValue("12");
 await page.getByLabel("Minimum password length in bytes").fill("24");state.writeStatus=503;await page.getByRole("button",{name:"Save Changes",exact:true}).click();const dialog=page.getByRole("alertdialog");await dialog.getByRole("button",{name:"Save Changes",exact:true}).click();await expect(dialog.getByRole("alert")).toBeVisible();await expect(page.getByLabel("Minimum password length in bytes")).toHaveValue("24");const key=state.writes[0].headers["idempotency-key"];state.writeStatus=200;await dialog.getByRole("button",{name:"Save Changes",exact:true}).click();await expect(dialog).toHaveCount(0);assert.equal(state.writes[1].headers["idempotency-key"],key);
 }finally{await browser.close()}
});
test("loading failures show no fake values and readers cannot edit",{timeout:60000},async()=>{
 const{browser,page,state}=await open(undefined,false);try{state.readStatus=503;await page.goto(origin+"/platform/settings/security");await expect(page.getByRole("button",{name:"Retry security settings"})).toBeVisible();await expect(page.getByLabel("Minimum password length in bytes")).toHaveCount(0);state.readStatus=200;await page.getByRole("button",{name:"Retry security settings"}).click();await expect(page.getByLabel("Minimum password length in bytes")).toHaveValue("12");await expect(page.getByLabel("Minimum password length in bytes")).toBeDisabled();assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
test("reset-password form uses current global minimum without real provider calls",{timeout:60000},async()=>{
 const browser=await chromium.launch({headless:true});try{const page=await browser.newPage();let writes=0;await page.route("**/api/**",route=>{const p=new URL(route.request().url()).pathname;if(p==="/api/auth/password-policy")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify(envelope({minimum_password_bytes:24,maximum_password_bytes:128}))});if(p==="/api/auth/password-reset"){writes++;return route.fulfill({status:204})}return route.fulfill({status:503,contentType:"application/json",body:"{}"})});await page.goto(origin+"/reset-password");await expect(page.getByText(/24–128 bytes/)).toBeVisible();await page.getByLabel("Recovery code",{exact:true}).fill("A".repeat(43));await page.getByLabel("New password",{exact:true}).fill("short password");await page.getByLabel("Confirm new password",{exact:true}).fill("short password");await page.getByRole("button",{name:"Reset password",exact:true}).click();assert.equal(writes,0);await expect(page.locator('[data-slot="field-error"]')).toContainText("longer password");await page.getByLabel("New password",{exact:true}).fill("this is a longer secure passphrase");await page.getByLabel("Confirm new password",{exact:true}).fill("this is a longer secure passphrase");await page.getByRole("button",{name:"Reset password",exact:true}).click();await expect(page).toHaveURL(/\/login$/);assert.equal(writes,1);}finally{await browser.close()}
});
