import assert from "node:assert/strict";
import test from "node:test";
import {chromium,expect} from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3000";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use loopback; every API is intercepted.");
const token="A".repeat(22)+".001234",flow="00000000-0000-4000-8000-000000000001",csrf="A".repeat(43);
for(const [name,status] of [["new invite",204],["unfinished account with an earlier password",204],["protected or used offer",409],["unconfirmed activation",503]])test(`${name}: password activation never becomes registration/recovery or inherits inviter access`,{timeout:45000},async()=>{
 const browser=await chromium.launch({headless:true});try{
  const page=await browser.newPage(),writes=[];let meReads=0;
  await page.route("**/api/**",async route=>{
   const request=route.request(),path=new URL(request.url()).pathname;
   if(path==="/api/auth/invitation-context")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{flow_id:flow}})});
   if(path==="/api/auth/invitation-details")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{email:"i***@e***.test",role_key:"PLATFORM_OWNER",setup_state:"SET_PASSWORD"}})});
   if(path==="/api/auth/password-policy")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{minimum_password_bytes:12,maximum_password_bytes:128}})});
   if(path==="/api/session/me"){
    meReads++;return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:["platform.admin.invite"],session:{assurance_level:"AAL2",idle_expires_at:"2030-01-01T00:00:00Z",absolute_expires_at:"2030-01-02T00:00:00Z"},required_actions:[]},meta:{trace_id:"fixture"}})});
   }
   if(path==="/api/session/bootstrap")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{csrf_token:csrf}})});
   if(path==="/api/auth/logout")return route.fulfill({status:204,body:""});
   if(request.method()==="POST")writes.push({path,body:request.postDataJSON(),key:request.headers()["idempotency-key"]});
   if(path==="/api/auth/invitation-activate")return route.fulfill(status===204?{status:204,body:""}:{status,contentType:"application/json",body:JSON.stringify({error:{code:status===409?"INVITATION_UNAVAILABLE":"ACTIVATION_UNCONFIRMED"}})});
   return route.fulfill({status:401,contentType:"application/json",body:"{}"});
  });
  await page.goto(origin+"/invitations/platform#code="+token);
  await expect(page.getByLabel("Email address",{exact:true})).toHaveValue("i***@e***.test");
  await expect(page.getByRole("button",{name:"Continue to my account",exact:true})).toHaveCount(0);
  await expect(page.getByRole("button",{name:"Use the invited account",exact:true})).toHaveCount(0);
  await expect(page.getByText("Sign in with your existing account to accept your invitation.",{exact:false})).toHaveCount(0);
  assert.equal(meReads,0,"opening the offer must not select a screen using the inviter's account");
  assert.equal(writes.length,0,"opening a link must never activate it");
  await page.getByLabel("Password",{exact:true}).fill("synthetic activation password");await page.getByLabel("Confirm password",{exact:true}).fill("synthetic activation password");
  await page.getByRole("button",{name:"Activate account",exact:true}).click();
  if(status===204)await expect(page).toHaveURL(origin+"/login");
  else{await expect(page.getByRole("alert")).toBeVisible();await expect(page).toHaveURL(origin+"/invitations/platform?invitation="+flow)}
  assert.equal(writes.length,1);assert.equal(writes[0].path,"/api/auth/invitation-activate");assert.deepEqual(writes[0].body,{token,password:"synthetic activation password"});assert.match(writes[0].key,/^[a-f0-9-]{36}$/);
 }finally{await browser.close()}
});
