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
async function open(viewport,reader=false,grants=caps){
 const browser=await chromium.launch({headless:true}),page=await browser.newPage({viewport});
 const state={users:[user(1,"owner@example.test","PLATFORM_OWNER"),user(2,"billing@example.test","BILLING_ADMINISTRATOR","DISABLED"),user(3,"support@example.test","SUPPORT_ADMINISTRATOR")],requests:[],writes:[],invitationStatus:201,directoryStatus:200,removeStatus:204,removeCode:"UNAVAILABLE",denied:false,ownerCount:1,invitations:[40,41,42].map(n=>({id:id(n),first_name:"Invited",last_name:String(n),email:`invited${n}@example.test`,role_key:"SUPPORT_ADMINISTRATOR",state:"ACTIVE",version:1,created_at:at,expires_at:"2026-11-11T12:00:00Z",delivery_state:"DELIVERY_UNCONFIRMED",governance_change_id:null}))};
 await page.route("**/api/**",async route=>{
  const req=route.request(),url=new URL(req.url()),p=url.pathname;state.requests.push({method:req.method(),url:req.url()});let data,status=200;
  if(p==="/api/session/bootstrap")data={data:{csrf_token:"A".repeat(43)},meta:{trace_id:id(99)}};
  else if(p==="/api/session/me")data={data:{account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:reader?["platform.admin.read"]:grants,required_actions:[],session:{assurance_level:"AAL2",idle_expires_at:new Date(Date.now()+3600000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()}},meta:{trace_id:id(99)}};
  else if(p==="/api/session/availability")data={data:{maintenance_active:false,maintenance_message:""},meta:{trace_id:id(99)}};
  else if(p==="/api/platform/administrators"||p==="/api/platform/administrators/search"){
   if(state.directoryStatus!==200){status=state.directoryStatus;data={error:{code:"UNAVAILABLE"}};}
   else if(state.denied){status=403;data={error:{code:"FORBIDDEN"}};}
   else{const body=req.method()==="POST"?req.postDataJSON():{},filter={state:body.state??url.searchParams.get("state"),role:body.role_key??url.searchParams.get("role_key"),email:body.email};
   const rows=state.users.filter(u=>(!filter.state||u.state===filter.state)&&(!filter.role||u.assignments.some(a=>a.state==="ACTIVE"&&a.role_key===filter.role))&&(!filter.email||u.email.toLowerCase()===filter.email.toLowerCase()));
   const more=!filter.email&&!filter.state&&!filter.role&&!url.searchParams.has("cursor")&&rows.length>2;
   data=wrap({administrators:url.searchParams.has("cursor")?rows.slice(2):more?rows.slice(0,2):rows,effective_owner_count:state.ownerCount,continuity_warning:state.ownerCount<2},more?"next-page":null);}
  }else if(/^\/api\/platform\/administrators\/[a-f0-9-]{36}(\/assignment-history)?$/.test(p)){
   const uid=p.split("/")[4],u=state.users.find(row=>row.user_id===uid);if(state.denied){status=403;data={error:{code:"FORBIDDEN"}};}else if(!u){status=404;data={error:{code:"NOT_FOUND"}};}else data=wrap(p.endsWith("/assignment-history")?{user_id:uid,assignments:u.assignments}:u);
  }else if(/^\/api\/platform\/administrator-invitations\/[a-f0-9-]{36}$/.test(p)&&req.method()==="DELETE"){
   state.writes.push({path:p,method:"DELETE",headers:req.headers()});status=state.removeStatus;
   const target=p.split("/").at(-1),row=state.invitations.find(row=>row.id===target);
   if(status===204&&(!row||req.headers()["if-match"]!==`"${row.version}"`))status=409;
   if(status===204)state.invitations=state.invitations.filter(row=>row.id!==target);else data={error:{code:state.removeCode}};
  }else if(p==="/api/platform/administrator-invitations"&&req.method()==="GET"){
   data=wrap({invitations:url.searchParams.has("cursor")?state.invitations.slice(2):state.invitations.slice(0,2)},!url.searchParams.has("cursor")&&state.invitations.length>2?"invite-next":null);
  }else if(p==="/api/platform/administrator-invitations"&&req.method()==="POST"){
   const body=req.postDataJSON();state.writes.push({path:p,body,headers:req.headers()});status=state.invitationStatus;
   const invitation={id:id(50),first_name:body.first_name,last_name:body.last_name,email:body.email,role_key:body.role_key,state:"ACTIVE",version:1,created_at:at,expires_at:"2026-11-11T12:00:00Z",delivery_state:"ACCEPTED_FOR_DELIVERY",governance_change_id:null};
   if(status===201)state.invitations.unshift(invitation);
   data=status===201?wrap(invitation):{error:{code:"UNAVAILABLE"}};
  }else if(p==="/api/platform/administrator-changes"&&req.method()==="POST"){
   const body=req.postDataJSON();state.writes.push({path:p,body,headers:req.headers()});const u=state.users.find(u=>u.user_id===body.target_user_id);
   if(body.action==="DELETE_IDENTITY")state.users=state.users.filter(row=>row.user_id!==u.user_id);else if(body.action==="DISABLE_IDENTITY")u.state="DISABLED";else if(body.action==="REACTIVATE_IDENTITY")u.state="ACTIVE";else if(body.action==="ASSIGN_ROLE")u.assignments.push(assignment(30,body.role_key));u.user_version++;u.platform_context_generation++;
   status=201;data=wrap({id:id(60),action:body.action,target_user_id:u.user_id,invitation_id:null,role_key:body.role_key??null,assignment_id:body.assignment_id??null,expected_user_version:body.expected_user_version,expected_target_context_generation:body.expected_target_context_generation,expected_assignment_version:body.expected_assignment_version??null,reason:body.reason,decision_reason:null,state:"EXECUTED",version:1,maker_id:id(1),checker_id:null,created_at:at,expires_at:null,decided_at:at,sessions_revoked:true});
  }else{status=503;data={error:{code:"UNAVAILABLE"}};}
  if(status===204)await route.fulfill({status,body:""});else await route.fulfill({status,contentType:"application/json",body:JSON.stringify(data)});
 });return{browser,page,state};
}
for(const [name,viewport]of [["desktop",{width:1280,height:900}],["mobile",{width:390,height:844}]])test("Users directory paginates, filters privately and retains authorized actions on "+name,{timeout:60000},async()=>{
 const{browser,page,state}=await open(viewport);try{
  await page.goto(origin+"/platform/users");
  await expect(page.getByRole("heading",{name:"Users",exact:true})).toBeVisible();
  await expect(page.getByText("Owner continuity needs attention",{exact:true})).toHaveCount(0);
  await expect(page.getByRole("link",{name:"Review approvals",exact:true})).toHaveCount(0);
  const pages=page.getByRole("navigation",{name:"Users pages",exact:true});
  await expect(page.getByRole("table").getByRole("row")).toHaveCount(3);
  await expect(page.getByRole("row").filter({hasText:"billing@example.test"})).toContainText("Disabled");
  await expect(pages.getByRole("button",{name:"Previous",exact:true})).toBeDisabled();
  await pages.getByRole("button",{name:"Next",exact:true}).click();
  await expect(page.getByRole("table").getByRole("row")).toHaveCount(2);
  await expect(page.getByRole("row").filter({hasText:"support@example.test"})).toBeVisible();
  await expect(pages.getByRole("button",{name:"Next",exact:true})).toBeDisabled();
  await pages.getByRole("button",{name:"Previous",exact:true}).click();
  await expect(page.getByRole("table").getByRole("row")).toHaveCount(3);
  await page.getByLabel("Status",{exact:true}).selectOption("DISABLED");await page.getByLabel("Role",{exact:true}).selectOption("BILLING_ADMINISTRATOR");
  await page.getByRole("button",{name:"Apply filters",exact:true}).click();await expect(page.getByRole("table").getByRole("row")).toHaveCount(2);
  const filtered=state.requests.filter(r=>r.url.includes("/api/platform/administrators?")).at(-1);
  assert.ok(filtered.url.includes("state=DISABLED")&&filtered.url.includes("role_key=BILLING_ADMINISTRATOR")&&!filtered.url.includes("cursor="));
  await page.getByLabel("Email address",{exact:true}).fill("BILLING@example.test");await page.getByRole("button",{name:"Apply filters",exact:true}).click();
  await expect.poll(()=>state.requests.some(r=>r.method==="POST"&&r.url.endsWith("/administrators/search"))).toBe(true);
  assert.equal(state.requests.some(r=>r.url.includes("@")||r.url.includes("%40")||r.url.includes("email=")),false);
  await page.getByRole("button",{name:"Clear filters",exact:true}).click();await expect(page.getByRole("table").getByRole("row")).toHaveCount(3);
  await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();
  await expect(page.getByRole("menuitem",{name:"View details",exact:true})).toBeVisible();
  await expect(page.getByRole("menuitem",{name:"Disable user",exact:true})).toHaveCount(0);
  await expect(page.getByRole("menuitem",{name:"Revoke sessions",exact:true})).toHaveCount(0);
  await page.keyboard.press("Escape");await expect(page.getByRole("menu")).toHaveCount(0);await pages.getByRole("button",{name:"Next",exact:true}).click();
  await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Disable user",exact:true}).click();
  const access=page.getByRole("alertdialog");await access.getByLabel("Reason",{exact:true}).fill("Synthetic access change");await access.getByRole("button",{name:"Disable administrator",exact:true}).click();
  await expect(access).toHaveCount(0);await expect(page.getByRole("row").filter({hasText:"support@example.test"})).toContainText("Disabled");assert.equal(state.writes.at(-1).body.action,"DISABLE_IDENTITY");
  await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Reactivate user",exact:true}).click();
  await access.getByLabel("Reason",{exact:true}).fill("Synthetic restore");await access.getByRole("button",{name:"Reactivate administrator",exact:true}).click();
  await expect(access).toHaveCount(0);await expect(page.getByRole("row").filter({hasText:"support@example.test"})).toContainText("Active");assert.equal(state.writes.at(-1).body.action,"REACTIVATE_IDENTITY");
  await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Manage roles",exact:true}).click();
  const sheet=page.getByRole("dialog");await sheet.getByLabel("Action",{exact:false}).selectOption("ASSIGN_ROLE");await sheet.getByLabel(/^Role/).selectOption("BILLING_ADMINISTRATOR");await sheet.getByLabel("Reason",{exact:false}).fill("Synthetic role change");await sheet.getByRole("button",{name:"Submit change",exact:true}).click();
  await expect(sheet).toHaveCount(0);assert.equal(state.writes.at(-1).body.role_key,"BILLING_ADMINISTRATOR");
  assert.equal(state.requests.some(r=>r.url.includes("administrator-changes?state=PENDING_APPROVAL")),false);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 }finally{await browser.close()}
});
test("Disabled user deletion requires typed confirmation and removes the row only after server success",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:1280,height:900});try{
  await page.goto(origin+"/platform/users");
  await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();await expect(page.getByRole("menuitem",{name:"Delete user",exact:true})).toHaveCount(0);await page.keyboard.press("Escape");
  await page.getByRole("button",{name:"Actions for billing@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Delete user",exact:true}).click();
  const dialog=page.getByRole("alertdialog");await expect(dialog.getByRole("heading",{name:"Delete this user?",exact:true})).toBeVisible();
  await dialog.getByLabel("Reason",{exact:true}).fill("Delete disabled fixture");
  await dialog.getByLabel("Confirm email address",{exact:true}).fill("wrong@example.test");
  await dialog.getByRole("button",{name:"Delete user",exact:true}).click();
  await expect(dialog.getByRole("alert")).toContainText("Type the user");assert.equal(state.writes.length,0);
  await dialog.getByLabel("Confirm email address",{exact:true}).fill("billing@example.test");
  await dialog.getByRole("button",{name:"Delete user",exact:true}).click();
  await expect(dialog).toHaveCount(0);await expect(page.getByRole("row").filter({hasText:"billing@example.test"})).toHaveCount(0);
  assert.equal(state.writes.length,1);assert.equal(state.writes[0].body.action,"DELETE_IDENTITY");assert.equal(state.writes[0].body.target_user_id,id(2));
  assert.ok(state.writes[0].headers["x-csrf-token"]);assert.ok(state.writes[0].headers["idempotency-key"]);
 }finally{await browser.close()}
});
test("Security auditor cannot see disabled user deletion",{timeout:60000},async()=>{
 const{browser,page}=await open({width:1280,height:900},false,["platform.admin.read","platform.admin.identity.disable"]);try{
  await page.goto(origin+"/platform/users");await page.getByRole("button",{name:"Actions for billing@example.test",exact:true}).click();await expect(page.getByRole("menuitem",{name:"Delete user",exact:true})).toHaveCount(0);
 }finally{await browser.close()}
});

test("Users directory keeps the audit-style table and filter card without Refresh or Retry",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:1280,height:900});try{
  state.users=[];await page.goto(origin+"/platform/users");
  await expect(page.getByRole("cell",{name:"No data.",exact:true})).toBeVisible();await expect(page.getByRole("columnheader")).toHaveCount(4);
  await expect(page.getByText(/All Platform Admin users, including disabled accounts|filters apply across all pages/)).toHaveCount(0);
  await expect(page.getByRole("button",{name:"Refresh",exact:true})).toHaveCount(0);
  state.directoryStatus=503;await page.getByRole("button",{name:"Apply filters",exact:true}).click();
  await expect(page.locator('section[data-state="error"]')).toBeVisible();await expect(page.getByRole("cell",{name:"No data.",exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Retry",exact:true})).toHaveCount(0);await expect(page.getByText("Users are temporarily unavailable.",{exact:true})).toHaveCount(0);
  await expect(page.getByLabel("User filters").getByRole("heading",{name:"Filters",exact:true})).toBeVisible();
  state.directoryStatus=200;await page.getByRole("button",{name:"Apply filters",exact:true}).click();await expect(page.locator('section[data-state="ready"]')).toBeVisible();
 }finally{await browser.close()}
});
test("Invite user requires names, uses Sonner, preserves retry identity and opens Invited without approvals",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:390,height:844});try{
  await page.goto(origin+"/platform/users");await page.getByRole("button",{name:"Invite User",exact:true}).click();
  const dialog=page.getByRole("alertdialog"),send=dialog.getByRole("button",{name:"Send invitation",exact:true});
  await dialog.getByLabel("Email address",{exact:true}).fill("invalid");await dialog.getByLabel("Role",{exact:true}).selectOption("PLATFORM_OWNER");await expect(send).toBeDisabled();
  await dialog.getByLabel("Email address",{exact:true}).fill("new@example.test");await expect(send).toBeDisabled();
  await dialog.getByLabel("First name",{exact:true}).fill("Amina");await dialog.getByLabel("Last name",{exact:true}).fill("Njoroge");
  await expect(dialog.getByLabel("Reason",{exact:true})).toHaveCount(0);await expect(dialog.getByText(/approval|reviewer|checker/i)).toHaveCount(0);
  state.invitationStatus=503;await send.click();await expect(page.getByText("Invitation service is unavailable. Retry with the same details.",{exact:true})).toBeVisible();
  await expect(dialog.getByLabel("First name",{exact:true})).toHaveValue("Amina");const key=state.writes[0].headers["idempotency-key"];
  state.invitationStatus=201;await send.click();await expect(dialog).toHaveCount(0);
  await expect(page.getByText("Invitation created. Email accepted for delivery.",{exact:true})).toBeVisible();
  assert.equal(state.writes[1].headers["idempotency-key"],key);
  assert.deepEqual(state.writes[1].body,{first_name:"Amina",last_name:"Njoroge",email:"new@example.test",role_key:"PLATFORM_OWNER"});
  await expect(page.getByRole("tab",{name:"Invited",exact:true})).toHaveAttribute("aria-selected","true");
  await expect(page.getByRole("row").filter({hasText:"new@example.test"})).toContainText("Amina Njoroge");
  const pages=page.getByRole("navigation",{name:"Invited users pages",exact:true});await pages.getByRole("button",{name:"Next",exact:true}).click();await expect(pages.getByText("Page 2",{exact:true})).toBeVisible();
  await pages.getByRole("button",{name:"Previous",exact:true}).click();await expect(page.getByRole("row").filter({hasText:"new@example.test"})).toBeVisible();
  await page.getByRole("tab",{name:"Users",exact:true}).click();await expect(page.getByLabel("User filters")).toBeVisible();
  assert.equal(state.writes.length,2);
 }finally{await browser.close()}
});
test("Invited user removal confirms cancellation and preserves retry identity",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:1280,height:900});try{
  await page.goto(origin+"/platform/users");await page.getByRole("tab",{name:"Invited",exact:true}).click();
  await page.getByRole("navigation",{name:"Invited users pages",exact:true}).getByRole("button",{name:"Next",exact:true}).click();
  await page.getByRole("button",{name:"Actions for invited42@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Remove invitation",exact:true}).click();
  const dialog=page.getByRole("alertdialog");await expect(dialog).toContainText("Existing user accounts are not deleted");
  state.removeStatus=503;await dialog.getByRole("button",{name:"Remove invitation",exact:true}).click();
  await expect(page.getByText("Removal could not be confirmed. Try again with the same invitation.",{exact:true})).toBeVisible();await expect(dialog).toBeVisible();
  const key=state.writes[0].headers["idempotency-key"];state.removeStatus=204;
  await dialog.getByRole("button",{name:"Remove invitation",exact:true}).click();await expect(dialog).toHaveCount(0);
  await expect(page.getByText("Invitation removed.",{exact:true})).toBeVisible();await expect(page.getByRole("row").filter({hasText:"invited42@example.test"})).toHaveCount(0);await expect(page.getByRole("navigation",{name:"Invited users pages",exact:true}).getByText("Page 1",{exact:true})).toBeVisible();
  assert.equal(state.writes[1].headers["idempotency-key"],key);assert.equal(state.writes[1].headers["if-match"],'"1"');assert.equal(state.writes[1].method,"DELETE");assert.equal(state.users.length,3);
 }finally{await browser.close()}
});
test("Owner invitation removal distinguishes expired verification from missing permission",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:1280,height:900});try{
  state.removeStatus=403;state.removeCode="PLATFORM_ASSURANCE_REQUIRED";
  await page.goto(origin+"/platform/users");await page.getByRole("tab",{name:"Invited",exact:true}).click();
  await page.getByRole("button",{name:"Actions for invited40@example.test",exact:true}).click();await page.getByRole("menuitem",{name:"Remove invitation",exact:true}).click();
  const dialog=page.getByRole("alertdialog");await dialog.getByRole("button",{name:"Remove invitation",exact:true}).click();
  await expect(page.getByText("Please sign out and sign in again to verify your account, then remove the invitation.",{exact:true})).toBeVisible();
  await expect(dialog).toBeVisible();assert.equal(state.invitations.length,3);
  state.removeCode="PLATFORM_ACCESS_FORBIDDEN";await dialog.getByRole("button",{name:"Remove invitation",exact:true}).click();
  await expect(page.getByText("Your current account does not have permission to remove this invitation.",{exact:true})).toBeVisible();assert.equal(state.invitations.length,3);
 }finally{await browser.close()}
});
test("Invited user removal is unavailable to readers",{timeout:60000},async()=>{
 const{browser,page,state}=await open(undefined,true);try{
  await page.goto(origin+"/platform/users");await page.getByRole("tab",{name:"Invited",exact:true}).click();
  await expect(page.getByRole("row").filter({hasText:"invited40@example.test"})).toBeVisible();await expect(page.getByRole("button",{name:/Actions for invited/})).toHaveCount(0);assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
for(const[name,viewport]of[["desktop",{width:1440,height:1000}],["mobile",{width:390,height:844}]])test("User details card layout preserves access controls on "+name,{timeout:60000},async()=>{
 const{browser,page,state}=await open(viewport);try{
  await page.goto(origin+"/platform/users/"+id(3));
  await expect(page.getByRole("heading",{name:"User details",exact:true})).toBeVisible();
  await expect(page.getByRole("region",{name:"Administrator account",exact:true})).toContainText("support@example.test");
  await expect(page.getByRole("heading",{name:"Manage roles",exact:true})).toBeVisible();await expect(page.getByRole("heading",{name:"Role history",exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Revoke sessions",exact:true})).toBeDisabled();await expect(page.getByRole("button",{name:"Disable administrator",exact:true})).toBeVisible();
  await page.getByRole("button",{name:"Disable administrator",exact:true}).click();await expect(page.getByRole("alertdialog")).toBeVisible();await page.getByRole("alertdialog").getByRole("button",{name:"Cancel",exact:true}).click();await expect(page.getByRole("alertdialog")).toHaveCount(0);assert.equal(state.writes.length,0);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  if(name==="desktop")await page.screenshot({path:process.env.HOME+"/.local/share/bidora-dev/user-details-cards.png"});
  await page.getByRole("link",{name:"Back to users",exact:true}).click();await expect(page).toHaveURL(origin+"/platform/users");
 }finally{await browser.close()}
});
test("User details readers see account and history without management controls",{timeout:60000},async()=>{
 const{browser,page,state}=await open(undefined,true);try{
  await page.goto(origin+"/platform/users/"+id(3));await expect(page.getByRole("region",{name:"Administrator account",exact:true})).toBeVisible();await expect(page.getByRole("heading",{name:"Role history",exact:true})).toBeVisible();
  await expect(page.getByRole("heading",{name:"Manage roles",exact:true})).toHaveCount(0);await expect(page.getByRole("button",{name:"Disable administrator",exact:true})).toHaveCount(0);await expect(page.getByRole("button",{name:"Revoke sessions",exact:true})).toHaveCount(0);assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
test("User navigation matches Tempo line tabs and supports keyboard switching",{timeout:60000},async()=>{
 const{browser,page}=await open({width:1440,height:1000});try{
  await page.goto(origin+"/platform/users");
  const tabs=page.getByRole("tablist",{name:"User directory views",exact:true});
  await expect(tabs).toHaveAttribute("data-variant","line");
  const users=page.getByRole("tab",{name:"Users",exact:true}),invited=page.getByRole("tab",{name:"Invited",exact:true});
  await expect(users).toHaveAttribute("aria-selected","true");
  await users.focus();await page.keyboard.press("ArrowRight");await page.keyboard.press("Enter");
  await expect(invited).toHaveAttribute("aria-selected","true");await expect(page.getByRole("region",{name:"Invited users",exact:true})).toBeVisible();
  await expect.poll(()=>invited.evaluate(el=>getComputedStyle(el,"::after").opacity)).toBe("1");
  await page.screenshot({path:process.env.HOME+"/.local/share/bidora-dev/users-tempo-tabs.png"});
  await invited.focus();await page.keyboard.press("ArrowLeft");await page.keyboard.press("Enter");
  await expect(users).toHaveAttribute("aria-selected","true");
  await expect(page.getByLabel("User filters")).toBeVisible();
 }finally{await browser.close()}
});
test("Security and Audit can disable ordinary admins but never an Owner directly",{timeout:60000},async()=>{
 const{browser,page,state}=await open(undefined,false,["platform.admin.read","platform.admin.identity.disable","platform.admin.session.revoke","platform.governance.review"]);try{
  state.ownerCount=2;await page.goto(origin+"/platform/users");await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();
  await expect(page.getByRole("menuitem",{name:"View details",exact:true})).toBeVisible();
  await expect(page.getByRole("menuitem",{name:"Disable user",exact:true})).toHaveCount(0);await expect(page.getByRole("menuitem",{name:"Revoke sessions",exact:true})).toHaveCount(0);
  await page.keyboard.press("Escape");await expect(page.getByRole("menu")).toHaveCount(0);await page.getByRole("navigation",{name:"Users pages",exact:true}).getByRole("button",{name:"Next",exact:true}).click();
  await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();await expect(page.getByRole("menuitem",{name:"Disable user",exact:true})).toBeVisible();assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
test("Readers cannot invite or mutate users and forbidden directory reads fail closed",{timeout:60000},async()=>{
 const{browser,page,state}=await open(undefined,true);try{
  await page.goto(origin+"/platform/users");await expect(page.getByRole("button",{name:"Actions for owner@example.test",exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Invite User",exact:true})).toHaveCount(0);await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();
  await expect(page.getByRole("menuitem")).toHaveCount(1);await expect(page.getByRole("menuitem",{name:"View details",exact:true})).toBeVisible();await page.keyboard.press("Escape");
  for(const path of ["/platform/users/approvals","/platform/approvals"]){await page.goto(origin+path);await expect(page).toHaveURL(origin+"/platform/users");}
  state.denied=true;await page.getByRole("button",{name:"Apply filters",exact:true}).click();await expect(page.locator("main").getByRole("alert")).toContainText("don’t have access");await expect(page.getByRole("table")).toHaveCount(0);assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
