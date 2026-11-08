import assert from "node:assert/strict";
import { chromium, expect } from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3187";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use a loopback test origin; all APIs are intercepted.");
const id="11111111-1111-4111-8111-111111111111",wrap=data=>({data,meta:{trace_id:id}});
const browser=await chromium.launch({headless:true});
try{
 for(const viewport of [{width:1280,height:900},{width:390,height:844}]){
  const page=await browser.newPage({viewport}),requests=[];let writes=0;
  await page.route("**/api/**",async route=>{
   const request=route.request(),path=new URL(request.url()).pathname;requests.push(path);
   let data,status=200;const now=new Date().toISOString(),expires=new Date(Date.now()+3600000).toISOString();
   if(request.method()!=="GET"){writes++;status=503;data={error:{code:"UNAVAILABLE"}};}
   else if(path==="/api/session/bootstrap")data=wrap({csrf_token:"A".repeat(43)});
   else if(path==="/api/session/me")data=wrap({account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:["platform.integration.failure.read"],session:{assurance_level:"AAL2",idle_expires_at:expires,absolute_expires_at:expires},required_actions:[]});
   else if(path==="/api/session/availability")data=wrap({maintenance_active:false,maintenance_message:""});
   else if(path==="/api/auth/password-policy")data=wrap({minimum_password_bytes:12,maximum_password_bytes:128});
   else if(path==="/api/session/sessions")data=wrap({active_session_count:1,other_session_count:0,sessions:[{session_id:id,rotated_from_session_id:null,current:true,created_at:now,last_seen_at:now,idle_expires_at:expires,absolute_expires_at:expires}],current_session_logout_available:true,other_session_revocation_available:false,revoke_all_available:true,revocation_policy_configured:true,device_metadata_collected:false,blockers:[],observed_at:now});
   else if(path==="/api/platform/communications/attempts")data={data:[],meta:{trace_id:id,next_cursor:null}};
   else{status=503;data={error:{code:"UNAVAILABLE"}};}
   await route.fulfill({status,contentType:"application/json",body:JSON.stringify(data)});
  });
  await page.goto(origin+"/account/security");
  await expect(page.getByRole("heading",{name:"Account security",exact:true,level:1})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Password",exact:true})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Active Sessions",exact:true})).toBeVisible();
  await expect(page.getByRole("rowheader",{name:"Current session",exact:true})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Email Verification",exact:true})).toHaveCount(0);
  await expect(page.getByRole("heading",{name:"Google sign-in",exact:true})).toHaveCount(0);
  await expect(page.getByText("Re-enter your password before continuing to Google. Matching email addresses are never linked automatically.",{exact:true})).toHaveCount(0);
  assert.equal(requests.some(path=>path.includes("mfa/preparation")||path.includes("google-link")),false);
  await page.goto(origin+"/platform/communications");
  await expect(page.getByRole("heading",{name:"Communications",exact:true})).toBeVisible();
  if(viewport.width<768)await page.getByRole("button",{name:"Collapse or expand navigation",exact:true}).click();
  const accountMenu=page.getByRole("button",{name:"Open administrator account menu",exact:true});
  await accountMenu.focus();await accountMenu.press("Enter");
  await expect(page.getByRole("menuitem",{name:"Account security",exact:true})).toBeVisible();
  await expect(page.getByRole("menuitem",{name:"Verify account again",exact:true})).toHaveCount(0);
  await expect(page.getByRole("menuitem",{name:"Sign out",exact:true})).toBeVisible();
  assert.equal(writes,0);await page.close();
 }
 console.log("PASS: desktop/mobile Account Security keeps password and active sessions, removes email/Google sections and requests, and removes only the sidebar re-verification link. All APIs intercepted; no real account action performed.");
}finally{await browser.close()}
