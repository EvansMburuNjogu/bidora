import assert from "node:assert/strict";
import test from "node:test";
import { chromium,expect } from "@playwright/test";

// Every API request is intercepted: no provider calls or persistence writes.
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3000";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use a loopback development origin.");
const oldID="11111111-1111-4111-8111-111111111111",activeID="22222222-2222-4222-8222-222222222222",changeID="33333333-3333-4333-8333-333333333333";
const at="2026-01-01T12:00:00Z";
const envelope=data=>({data,meta:{trace_id:changeID}});
const config={setting_kind:"SMTP",host:"smtp.example.test",port:587,credential_reference_configured:true,trust_roots_reference_configured:true,starttls_required:true,sender_name:"Synthetic Sender",sender_address:"sender@example.test",reply_to_address:null,connect_timeout_seconds:10,command_timeout_seconds:15,delivery_timeout_seconds:30,max_concurrency:4};
const old={id:oldID,setting_kind:"SMTP",sequence:1,state:"SUPERSEDED",version:5,configuration:config,test_status:"PASSED",test_code:"SMTP_STARTTLS_VERIFIED",created_at:at,tested_at:at,activated_at:at,superseded_at:at};
const active={...old,id:activeID,sequence:2,state:"ACTIVE",superseded_at:null};
const failure={error:{code:"SETTING_STATE_CONFLICT",message:"must not display upstream diagnostics",retryable:false,field_errors:[],trace_id:changeID}};
async function open({rollback=true,passed=true}={}){
 const browser=await chromium.launch({headless:true}),page=await browser.newPage();
 const caps=["platform.admin.read","platform.integration.read","platform.integration.smtp.activate","platform.integration.smtp.test",...(rollback?["platform.integration.rollback"]:[])];
 await page.route("**/api/**",async route=>{const path=new URL(route.request().url()).pathname;let data,status=200;
  if(path==="/api/session/bootstrap")data=envelope({csrf_token:"A".repeat(43)});
  else if(path==="/api/session/me")data=envelope({account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:caps,session:{assurance_level:"AAL2",idle_expires_at:new Date(Date.now()+600000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()},required_actions:[]});
  else if(path==="/api/session/availability")data=envelope({maintenance_active:false,maintenance_message:""});
  else if(path==="/api/platform/smtp")data=envelope(active);
  else if(path==="/api/platform/smtp/versions")data=envelope([active,{...old,test_status:passed?"PASSED":"FAILED",test_code:passed?"SMTP_STARTTLS_VERIFIED":"CONNECTIVITY_FAILED"}]);
  else{data=failure;status=503;}
  await route.fulfill({status,contentType:"application/json",body:JSON.stringify(data)});
 });return {browser,page};
}
test("SMTP rollback proposes the exact replaced version, preserves failures and awaits a distinct checker",{timeout:45000},async()=>{
 const {browser,page}=await open();let calls=0,status=409,release;const gate=new Promise(resolve=>{release=resolve;});
 try{
  await page.route("**/api/platform/smtp/propose",async route=>{calls++;const body=route.request().postDataJSON();assert.equal(body.action,"ROLLBACK");assert.equal(body.version_id,oldID);assert.equal(body.version,5);assert.equal(route.request().headers()["x-bidora-platform-context-generation"],"1");assert.ok(route.request().headers()["x-csrf-token"]);if(calls===1)await gate;await route.fulfill({status,contentType:"application/json",body:JSON.stringify(status===201?envelope({id:changeID,setting_kind:"SMTP",target_version_id:oldID,action:"ROLLBACK",state:"PENDING_APPROVAL",version:1,maker_user_id:activeID,checker_user_id:null,created_at:at,expires_at:"2030-01-01T12:30:00Z",decided_at:null}):failure)});});
  await page.goto(origin+"/platform/settings/smtp");await page.getByRole("button",{name:"Request rollback",exact:true}).click();const dialog=page.getByRole("alertdialog");await expect(dialog).toContainText("current active setting stays unchanged until approval");
  const confirm=dialog.getByRole("button",{name:"Request approval",exact:true});await confirm.evaluate(button=>{button.click();button.click();});await expect.poll(()=>calls).toBe(1);await expect(confirm).toBeDisabled();release();await expect(confirm).toBeEnabled();await expect(dialog).toBeVisible();await expect(page.getByText("must not display upstream diagnostics")).toHaveCount(0);
  status=201;await confirm.click();await expect(dialog).toHaveCount(0);await expect(page.locator('[data-sonner-toast][data-type="info"]')).toContainText("Rollback requested");await expect(page.locator('[data-sonner-toast][data-type="success"]')).toHaveCount(0);assert.equal(calls,2);await expect(page.getByText("Active managed setting · delivery not confirmed",{exact:true})).toBeVisible();
 }finally{release();await browser.close();}
});
for(const [name,options]of [["without rollback capability",{rollback:false}],["without a passing test",{passed:false}]])test(`SMTP rollback is unavailable ${name}`,{timeout:45000},async()=>{const {browser,page}=await open(options);try{await page.goto(origin+"/platform/settings/smtp");await expect(page.getByRole("region",{name:"Selected SMTP version"})).toBeVisible();await expect(page.getByRole("button",{name:"Request rollback",exact:true})).toHaveCount(0);}finally{await browser.close();}});
