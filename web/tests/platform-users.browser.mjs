import assert from "node:assert/strict";
import test from "node:test";
import { chromium, expect } from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3191";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use loopback; all APIs are intercepted.");
const id=n=>`${String(n).padStart(8,"0")}-1111-4111-8111-111111111111`,at="2026-11-08T12:00:00Z";
const wrap=(data,next_cursor=null)=>({data,meta:{trace_id:id(99),next_cursor}});
const assignment=(n,role_key)=>({id:id(n+10),role_key,state:"ACTIVE",version:1,assigned_by:id(1),assigned_at:at,revoked_by:null,revoked_at:null,reason:"Synthetic assignment",revocation_reason:null});
const user=(n,email,role,state="ACTIVE")=>({user_id:id(n),email,state,user_version:1,platform_context_generation:1,totp_enrolled:false,assignments:[assignment(n,role)],effective_owner:role==="PLATFORM_OWNER",created_at:at,updated_at:at});
const caps=["platform.admin.read","platform.admin.invite","platform.admin.role.assign","platform.admin.role.revoke","platform.admin.identity.disable","platform.admin.identity.reactivate","platform.admin.session.revoke","platform.governance.approve"];
async function open(viewport,reader=false){
 const browser=await chromium.launch({headless:true}),page=await browser.newPage({viewport});
 const state={users:[user(1,"owner@example.test","PLATFORM_OWNER"),user(2,"billing@example.test","BILLING_ADMINISTRATOR","DISABLED"),user(3,"support@example.test","SUPPORT_ADMINISTRATOR")],requests:[],writes:[],invitationStatus:201,denied:false};
 await page.route("**/api/**",async route=>{
  const req=route.request(),url=new URL(req.url()),p=url.pathname;state.requests.push({method:req.method(),url:req.url()});let data,status=200;
  if(p==="/api/session/bootstrap")data={data:{csrf_token:"A".repeat(43)},meta:{trace_id:id(99)}};
  else if(p==="/api/session/me")data={data:{account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:reader?["platform.admin.read"]:caps,required_actions:[],session:{assurance_level:"AAL2",idle_expires_at:new Date(Date.now()+3600000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()}},meta:{trace_id:id(99)}};
  else if(p==="/api/session/availability")data={data:{maintenance_active:false,maintenance_message:""},meta:{trace_id:id(99)}};
  else if(p==="/api/platform/administrators"||p==="/api/platform/administrators/search"){
   if(state.denied){status=403;data={error:{code:"FORBIDDEN"}};}
   else{const body=req.method()==="POST"?req.postDataJSON():{},filter={state:body.state??url.searchParams.get("state"),role:body.role_key??url.searchParams.get("role_key"),email:body.email};
   const rows=state.users.filter(u=>(!filter.state||u.state===filter.state)&&(!filter.role||u.assignments.some(a=>a.state==="ACTIVE"&&a.role_key===filter.role))&&(!filter.email||u.email.toLowerCase()===filter.email.toLowerCase()));
   const more=!filter.email&&!filter.state&&!filter.role&&!url.searchParams.has("cursor")&&rows.length>2;
   data=wrap({administrators:url.searchParams.has("cursor")?rows.slice(1):more?rows.slice(0,2):rows,effective_owner_count:1,continuity_warning:true},more?"next-page":null);}
  }else if(p==="/api/platform/administrator-invitations"&&req.method()==="POST"){
   const body=req.postDataJSON();state.writes.push({path:p,body,headers:req.headers()});status=state.invitationStatus;
   data=status===201?wrap({id:id(50),email:body.email,role_key:body.role_key,state:"ACTIVE",version:1,created_at:at,expires_at:"2026-11-11T12:00:00Z",delivery_state:"ACCEPTED_FOR_DELIVERY",governance_change_id:null}):{error:{code:"UNAVAILABLE"}};
  }else if(p==="/api/platform/administrator-changes"&&req.method()==="POST"){
   const body=req.postDataJSON();state.writes.push({path:p,body,headers:req.headers()});const u=state.users.find(u=>u.user_id===body.target_user_id);
   if(body.action==="DISABLE_IDENTITY")u.state="DISABLED";else if(body.action==="REACTIVATE_IDENTITY")u.state="ACTIVE";else if(body.action==="ASSIGN_ROLE")u.assignments.push(assignment(30,body.role_key));u.user_version++;u.platform_context_generation++;
   status=201;data=wrap({id:id(60),action:body.action,target_user_id:u.user_id,invitation_id:null,role_key:body.role_key??null,assignment_id:body.assignment_id??null,expected_user_version:body.expected_user_version,expected_target_context_generation:body.expected_target_context_generation,expected_assignment_version:body.expected_assignment_version??null,reason:body.reason,decision_reason:null,state:"EXECUTED",version:1,maker_id:id(1),checker_id:null,created_at:at,expires_at:null,decided_at:at,sessions_revoked:true});
  }else{status=503;data={error:{code:"UNAVAILABLE"}};}
  await route.fulfill({status,contentType:"application/json",body:JSON.stringify(data)});
 });return{browser,page,state};
}
for(const [name,viewport]of [["desktop",{width:1280,height:900}],["mobile",{width:390,height:844}]])test(`Users directory lists all states, pages, filters privately and offers real actions on ${name}`,{timeout:60000},async()=>{
 const{browser,page,state}=await open(viewport);try{
  await page.goto(origin+"/platform/users");await expect(page.getByRole("heading",{name:"Users",exact:true})).toBeVisible();await expect(page.getByRole("link",{name:"Review approvals",exact:true})).toHaveCount(0);
  await expect(page.getByRole("table").getByRole("row")).toHaveCount(3);await expect(page.getByRole("row").filter({hasText:"billing@example.test"})).toContainText("Disabled");
  await page.getByRole("button",{name:"Load more users",exact:true}).click();await expect(page.getByRole("table").getByRole("row")).toHaveCount(4);await expect(page.getByRole("button",{name:"Load more users",exact:true})).toHaveCount(0);
  await page.getByLabel("Status",{exact:true}).selectOption("DISABLED");await page.getByLabel("Role",{exact:true}).selectOption("BILLING_ADMINISTRATOR");await page.getByRole("button",{name:"Apply filters",exact:true}).click();await expect(page.getByRole("table").getByRole("row")).toHaveCount(2);
  const filtered=state.requests.filter(r=>r.url.includes("/api/platform/administrators?")).at(-1);assert.ok(filtered.url.includes("state=DISABLED")&&filtered.url.includes("role_key=BILLING_ADMINISTRATOR")&&!filtered.url.includes("cursor="));
  await page.getByLabel("Email address",{exact:true}).fill("BILLING@example.test");await page.getByRole("button",{name:"Apply filters",exact:true}).click();await expect(page.getByText("Matching email: BILLING@example.test.",{exact:false})).toBeVisible();assert.ok(state.requests.some(r=>r.method==="POST"&&r.url.endsWith("/administrators/search")));assert.equal(state.requests.some(r=>r.url.includes("@")||r.url.includes("%40")||r.url.includes("email=")),false);
  await page.getByRole("button",{name:"Clear filters",exact:true}).first().click();await expect(page.getByRole("button",{name:"Load more users",exact:true})).toBeVisible();await page.getByRole("button",{name:"Load more users",exact:true}).click();await expect(page.getByRole("row").filter({hasText:"support@example.test"})).toBeVisible();
  await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Disable user",exact:true}).click();const access=page.getByRole("alertdialog");await access.getByLabel("Reason",{exact:true}).fill("Synthetic access change");await access.getByRole("button",{name:"Disable administrator",exact:true}).click();await expect(access).toHaveCount(0);await expect(page.getByRole("button",{name:"Load more users",exact:true})).toBeVisible();assert.equal(state.writes.at(-1).body.action,"DISABLE_IDENTITY");
  await page.getByRole("button",{name:"Load more users",exact:true}).click();await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Reactivate user",exact:true}).click();await access.getByLabel("Reason",{exact:true}).fill("Synthetic restore");await access.getByRole("button",{name:"Reactivate administrator",exact:true}).click();await expect(access).toHaveCount(0);assert.equal(state.writes.at(-1).body.action,"REACTIVATE_IDENTITY");
  await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();await expect(page.getByRole("menuitem",{name:"View details",exact:true})).toBeVisible();await expect(page.getByRole("menuitem",{name:"Disable user",exact:true})).toHaveCount(0);await page.keyboard.press("Escape");await expect(page.getByRole("menu")).toHaveCount(0);await page.getByRole("button",{name:"Load more users",exact:true}).click();await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Manage roles",exact:true}).click();const sheet=page.getByRole("dialog");await sheet.getByLabel("Action",{exact:false}).selectOption("ASSIGN_ROLE");await sheet.getByLabel(/^Role/).selectOption("BILLING_ADMINISTRATOR");await sheet.getByLabel("Reason",{exact:false}).fill("Synthetic role change");await sheet.getByRole("button",{name:"Submit change",exact:true}).click();await expect(sheet).toHaveCount(0);assert.equal(state.writes.at(-1).body.role_key,"BILLING_ADMINISTRATOR");
  assert.equal(state.requests.some(r=>r.url.includes("administrator-changes?state=PENDING_APPROVAL")),false);assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 }finally{await browser.close()}
});
test("Invite user validates, retains retry identity and issues a direct Owner invitation",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:390,height:844});try{
  await page.goto(origin+"/platform/users");await page.getByRole("button",{name:"Invite User",exact:true}).click();const dialog=page.getByRole("alertdialog");await dialog.getByLabel("Email address",{exact:true}).fill("invalid");await dialog.getByLabel("Role",{exact:true}).selectOption("PLATFORM_OWNER");await dialog.getByLabel("Reason",{exact:true}).fill("Synthetic invitation");await expect(dialog.getByRole("button",{name:"Issue invitation",exact:true})).toBeDisabled();
  await dialog.getByLabel("Email address",{exact:true}).fill("new@example.test");state.invitationStatus=503;await dialog.getByRole("button",{name:"Issue invitation",exact:true}).click();await expect(dialog.getByRole("status")).toContainText("unavailable");const key=state.writes[0].headers["idempotency-key"];state.invitationStatus=201;await dialog.getByRole("button",{name:"Issue invitation",exact:true}).click();await expect(dialog.getByRole("status")).toContainText("accepted for delivery");assert.equal(state.writes[1].headers["idempotency-key"],key);assert.equal(state.writes[1].body.role_key,"PLATFORM_OWNER");await expect(dialog.getByText(/approval|reviewer|checker/i)).toHaveCount(0);await dialog.getByRole("button",{name:"Close",exact:true}).click();assert.equal(state.writes.length,2);
 }finally{await browser.close()}
});
test("Readers cannot invite or mutate users and forbidden directory reads fail closed",{timeout:60000},async()=>{
 const{browser,page,state}=await open(undefined,true);try{
  await page.goto(origin+"/platform/users");await expect(page.getByRole("button",{name:"Actions for owner@example.test",exact:true})).toBeVisible();await expect(page.getByRole("button",{name:"Invite User",exact:true})).toHaveCount(0);await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();await expect(page.getByRole("menuitem")).toHaveCount(1);await expect(page.getByRole("menuitem",{name:"View details",exact:true})).toBeVisible();await page.keyboard.press("Escape");state.denied=true;await page.getByRole("button",{name:"Refresh",exact:true}).click();await expect(page.locator("main").getByRole("alert")).toContainText("don’t have access");await expect(page.getByRole("table")).toHaveCount(0);assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
