import assert from "node:assert/strict";
import test from "node:test";
import { mkdtemp, rm, chmod } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { chromium, expect } from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3193";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use an isolated loopback origin. All APIs are intercepted.");
const id="11111111-1111-4111-8111-111111111111",meta={trace_id:id},wrap=data=>({data,meta});
const fresh=()=>({valid:true,bootstrapStatus:200,renewals:0,bootstraps:0,me:0,requests:[],renewHeaders:[],base:Date.now(),absolute:Date.now()+3600000,idle:Date.now()+300000});
async function intercept(page,state){
 await page.route("**/api/**",async route=>{
  const req=route.request(),path=new URL(req.url()).pathname,headers=await req.allHeaders();state.requests.push(path);let status=200,data;
  const valid=state.valid&&(headers.cookie??"").includes("session_audit_fixture=persistent");
  if(path==="/api/session/bootstrap"){state.bootstraps++;status=valid?state.bootstrapStatus:401;data=status===200?wrap({csrf_token:"A".repeat(43)}):{error:{code:status===401?"SESSION_INVALID":"AUTHENTICATION_UNAVAILABLE"}};}
  else if(path==="/api/session/me"){state.me++;status=valid?200:401;data=valid?wrap({account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:["platform.audit.read"],required_actions:[],session:{assurance_level:"AAL2",idle_expires_at:new Date(state.idle).toISOString(),absolute_expires_at:new Date(state.absolute).toISOString()}}):{error:{code:"SESSION_INVALID"}};}
  else if(path==="/api/session/renew"){
   state.renewals++;state.renewHeaders.push(headers);status=valid?200:401;
   state.idle=Math.min(state.absolute,Date.now()+300000);
   data=valid?wrap({idle_expires_at:new Date(state.idle).toISOString(),absolute_expires_at:new Date(state.absolute).toISOString(),observed_at:new Date().toISOString()}):{error:{code:"SESSION_INVALID"}};
  }else if(path==="/api/auth/logout"){state.valid=false;status=204;}
  else if(path==="/api/auth/password-policy")data=wrap({minimum_password_bytes:12,maximum_password_bytes:128});
  else if(path==="/api/session/availability")data=wrap({maintenance_active:false,maintenance_message:""});
  else if(path==="/api/session/sessions")data=wrap({active_session_count:1,other_session_count:0,sessions:[{session_id:id,rotated_from_session_id:null,current:true,created_at:new Date(state.base).toISOString(),last_seen_at:new Date(state.base).toISOString(),idle_expires_at:new Date(state.idle).toISOString(),absolute_expires_at:new Date(state.absolute).toISOString()}],current_session_logout_available:true,other_session_revocation_available:false,revoke_all_available:true,revocation_policy_configured:true,device_metadata_collected:false,blockers:[],observed_at:new Date().toISOString()});
  else if(path==="/api/platform/audit/actions")data={data:[],meta:{trace_id:id,next_cursor:null}};
  else{status=503;data={error:{code:"UNAVAILABLE"}};}
  await route.fulfill({status,headers:{"Content-Type":"application/json",Date:new Date().toUTCString()},body:status===204?undefined:JSON.stringify(data)});
 });
}
async function cookie(context){await context.addCookies([{name:"session_audit_fixture",value:"persistent",url:origin,httpOnly:true,sameSite:"Lax",expires:Math.floor(Date.now()/1000)+3600}]);}
async function open(){const browser=await chromium.launch({headless:true}),context=await browser.newContext();await cookie(context);const page=await context.newPage(),state=fresh();await intercept(page,state);return{browser,context,page,state};}
async function ready(page){await expect(page.getByRole("heading",{name:"Account security",exact:true})).toBeVisible();await expect(page.getByLabel("Current password",{exact:true})).toBeVisible();}

test("reload and normal protected navigation retain authentication without verification requests",{timeout:60000},async()=>{
 const{browser,page,state}=await open();try{
  await page.goto(origin+"/account/security");await ready(page);await page.reload();await ready(page);
  await page.goto(origin+"/platform/audit");await expect(page.getByRole("heading",{name:"Audit Trails",exact:true})).toBeVisible();await expect(page.getByText("No activity found.",{exact:true})).toBeVisible();
  assert.equal(state.requests.some(path=>/login-code|mfa|verify-email/.test(path)),false);assert.equal(state.renewals,0);
 }finally{await browser.close()}
});
test("trusted activity silently renews once, concurrent interactions do not rotate or overrun the lease",{timeout:60000},async()=>{
 const{browser,page,state}=await open();try{
  await page.clock.install();await page.goto(origin+"/account/security");await ready(page);
  const field=page.getByLabel("Current password",{exact:true});await field.click();await field.fill("synthetic retained form input");await page.clock.runFor(61000);
  await expect.poll(()=>state.renewals).toBe(1);
  for(let i=0;i<5;i++)await field.click();
  assert.equal(state.renewals,1);assert.equal(state.renewHeaders[0]["x-csrf-token"],"A".repeat(43));
  await expect(field).toHaveValue("synthetic retained form input");assert.equal(state.requests.some(path=>path.includes("login-code")),false);
  assert.ok(state.idle<=state.absolute);
 }finally{await browser.close()}
});
test("an idle tab never sends keepalive and genuine server expiry redirects to login",{timeout:60000},async()=>{
 const{browser,page,state}=await open();try{
  await page.clock.install();await page.goto(origin+"/account/security");await ready(page);
  state.valid=false;await page.clock.runFor(302000);await expect(page).toHaveURL(/\/login$/);assert.equal(state.renewals,0);
 }finally{await browser.close()}
});
test("transient session reads preserve valid authentication and unsaved inputs instead of logging out",{timeout:60000},async()=>{
 const{browser,page,state}=await open();try{
  await page.goto(origin+"/account/security");await ready(page);const field=page.getByLabel("Current password",{exact:true});await field.fill("synthetic unsaved input");
  state.bootstrapStatus=503;await page.evaluate(()=>document.dispatchEvent(new Event("visibilitychange")));
  await expect(page.getByText(/Connection interrupted/)).toBeVisible();await expect(field).toHaveValue("synthetic unsaved input");await expect(page).toHaveURL(/account\/security$/);
  state.bootstrapStatus=200;await page.getByRole("button",{name:"Retry",exact:true}).first().click();await expect(page.getByText(/Connection interrupted/)).toHaveCount(0);await expect(field).toHaveValue("synthetic unsaved input");
 }finally{await browser.close()}
});
test("logout synchronizes across tabs and authoritative revocation cannot silently renew",{timeout:60000},async()=>{
 const{browser,context,page,state}=await open();try{
  await page.goto(origin+"/account/security");await ready(page);const second=await context.newPage();await intercept(second,state);await second.goto(origin+"/account/security");await ready(second);
  await page.getByRole("button",{name:"Sign out",exact:true}).click();await expect(page).toHaveURL(/\/login$/);await expect(second).toHaveURL(/\/login$/);
  assert.equal(state.valid,false);
 }finally{await browser.close()}
});
test("server time prevents a skewed browser clock from causing an authentication refresh loop",{timeout:60000},async()=>{
 const{browser,page,state}=await open();try{
  await page.clock.install({time:new Date(Date.now()+2*3600000)});await page.goto(origin+"/account/security");await ready(page);
  const bootstraps=state.bootstraps,me=state.me;await page.clock.runFor(3000);assert.equal(state.bootstraps,bootstraps);assert.equal(state.me,me);await expect(page).toHaveURL(/account\/security$/);
 }finally{await browser.close()}
});
test("a persistent cookie restores authentication after the browser process restarts",{timeout:60000},async()=>{
 const dir=await mkdtemp(join(tmpdir(),"bidora-session-browser-"));await chmod(dir,0o700);let context;const state=fresh();
 try{
  context=await chromium.launchPersistentContext(dir,{headless:true});await cookie(context);let page=await context.newPage();await intercept(page,state);await page.goto(origin+"/account/security");await ready(page);await context.close();context=null;
  context=await chromium.launchPersistentContext(dir,{headless:true});page=await context.newPage();await intercept(page,state);await page.goto(origin+"/account/security");await ready(page);assert.equal(state.renewals,0);
 }finally{await context?.close();await rm(dir,{recursive:true,force:true})}
});
