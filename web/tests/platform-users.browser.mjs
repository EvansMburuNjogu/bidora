import assert from "node:assert/strict";
import test from "node:test";
import { chromium, expect } from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3191";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use loopback; all APIs are intercepted.");
const id=n=>`${String(n).padStart(8,"0")}-1111-4111-8111-111111111111`,at="2026-11-08T12:00:00Z";
const wrap=(data,next_cursor=null)=>({data,meta:{trace_id:id(99),...(next_cursor?{next_cursor}:{})}});
const assignment=(n,role_key)=>({id:id(n+10),role_key,state:"ACTIVE",version:1,assigned_by:id(1),assigned_at:at,revoked_by:null,revoked_at:null,reason:"Synthetic assignment",revocation_reason:null});
const user=(n,email,role,state="ACTIVE")=>({user_id:id(n),email,state,first_name:"Test",last_name:"User "+n,user_version:1,platform_context_generation:1,totp_enrolled:false,assignments:[assignment(n,role)],effective_owner:role==="PLATFORM_OWNER",created_at:at,updated_at:at});
const caps=["platform.admin.read","platform.admin.invite","platform.admin.role.assign","platform.admin.role.revoke","platform.admin.identity.disable","platform.admin.identity.reactivate","platform.admin.session.revoke","platform.governance.approve"];
async function open(viewport,reader=false,grants=caps){
 const browser=await chromium.launch({headless:true}),page=await browser.newPage({viewport});
 const state={users:[user(1,"owner@example.test","PLATFORM_OWNER"),user(2,"billing@example.test","BILLING_ADMINISTRATOR","DISABLED"),user(3,"support@example.test","SUPPORT_ADMINISTRATOR")],requests:[],writes:[],invitationStatus:201,denied:false,ownerCount:1,invitations:[]};
 await page.route("**/api/**",async route=>{
  const req=route.request(),url=new URL(req.url()),p=url.pathname;state.requests.push({method:req.method(),url:req.url()});let data,status=200;
  if(p==="/api/session/bootstrap")data={data:{csrf_token:"A".repeat(43)},meta:{trace_id:id(99)}};
  else if(p==="/api/session/me")data={data:{account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:reader?["platform.admin.read"]:grants,required_actions:[],session:{assurance_level:"AAL2",idle_expires_at:new Date(Date.now()+3600000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()}},meta:{trace_id:id(99)}};
  else if(p==="/api/session/availability")data={data:{maintenance_active:false,maintenance_message:""},meta:{trace_id:id(99)}};
  else if(p==="/api/platform/administrators"||p==="/api/platform/administrators/search"){
   if(state.denied){status=403;data={error:{code:"FORBIDDEN"}};}
   else{const body=req.method()==="POST"?req.postDataJSON():{},filter={state:body.state??url.searchParams.get("state"),role:body.role_key??url.searchParams.get("role_key"),email:body.email};
   const rows=state.users.filter(u=>(!filter.state||u.state===filter.state)&&(!filter.role||u.assignments.some(a=>a.state==="ACTIVE"&&a.role_key===filter.role))&&(!filter.email||u.email.toLowerCase()===filter.email.toLowerCase()));
   const more=!filter.email&&!filter.state&&!filter.role&&!url.searchParams.has("cursor")&&rows.length>2;
   data=wrap({administrators:url.searchParams.has("cursor")?rows.slice(2):more?rows.slice(0,2):rows,effective_owner_count:state.ownerCount,continuity_warning:state.ownerCount<2},more?"next-page":null);}
  }else if(p==="/api/platform/administrator-invitations"&&req.method()==="GET"){data={data:{invitations:state.invitations},meta:{trace_id:id(99),next_cursor:null}};
  }else if(p==="/api/platform/administrator-invitations"&&req.method()==="POST"){
   const body=req.postDataJSON();state.writes.push({path:p,body,headers:req.headers()});status=state.invitationStatus;
   data=status===201?wrap({id:id(50),email:body.email,role_key:body.role_key,state:"ACTIVE",version:1,created_at:at,expires_at:"2026-11-11T12:00:00Z",delivery_state:"ACCEPTED_FOR_DELIVERY",governance_change_id:null}):{error:{code:"UNAVAILABLE"}};
  }else if(p==="/api/platform/administrator-changes"&&req.method()==="POST"){
   const body=req.postDataJSON();state.writes.push({path:p,body,headers:req.headers()});const u=state.users.find(u=>u.user_id===body.target_user_id);
   if(body.action==="REMOVE_IDENTITY")state.users=state.users.filter(item=>item.user_id!==u.user_id);else if(body.action==="DISABLE_IDENTITY")u.state="DISABLED";else if(body.action==="REACTIVATE_IDENTITY")u.state="ACTIVE";else if(body.action==="ASSIGN_ROLE")u.assignments.push(assignment(30,body.role_key));u.user_version++;u.platform_context_generation++;
   status=201;data=wrap({id:id(60),action:body.action,target_user_id:u.user_id,invitation_id:null,role_key:body.role_key??null,assignment_id:body.assignment_id??null,expected_user_version:body.expected_user_version,expected_target_context_generation:body.expected_target_context_generation,expected_assignment_version:body.expected_assignment_version??null,reason:body.reason,decision_reason:null,state:"EXECUTED",version:1,maker_id:id(1),checker_id:null,created_at:at,expires_at:null,decided_at:at,sessions_revoked:true});
  }else{status=503;data={error:{code:"UNAVAILABLE"}};}
  await route.fulfill({status,contentType:"application/json",body:JSON.stringify(data)});
 });return{browser,page,state};
}

for(const [name,viewport]of [["desktop",{width:1280,height:900}],["mobile",{width:390,height:844}]])test(`Users table and pagination support all users, filters and guarded actions on ${name}`,{timeout:60000},async()=>{
 const{browser,page,state}=await open(viewport);try{
  await page.goto(origin+"/platform/users");
  await expect(page.getByRole("heading",{name:"Users",exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Refresh",exact:true})).toHaveCount(0);
  await expect(page.getByText(/All Platform Admin users, including disabled/)).toHaveCount(0);
  await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("table").getByRole("row")).toHaveCount(3);
  await expect(page.getByRole("row").filter({hasText:"billing@example.test"})).toContainText("Disabled");
  await page.getByRole("button",{name:"Next",exact:true}).click();
  await expect(page.getByText("Page 2",{exact:true})).toBeVisible();
  await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("table").getByRole("row")).toHaveCount(2);
  await expect(page.getByRole("row").filter({hasText:"support@example.test"})).toBeVisible();
  await page.getByRole("button",{name:"Previous",exact:true}).click();
  await expect(page.getByRole("row").filter({hasText:"owner@example.test"})).toBeVisible();
  await page.getByLabel("Status",{exact:true}).selectOption("DISABLED");
  await page.getByLabel("Role",{exact:true}).selectOption("BILLING_ADMINISTRATOR");
  await page.getByRole("button",{name:"Apply filters",exact:true}).click();
  await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("table").getByRole("row")).toHaveCount(2);
  await page.getByLabel("Email address",{exact:true}).fill("BILLING@example.test");
  await page.getByRole("button",{name:"Apply filters",exact:true}).click();
  await expect.poll(()=>state.requests.some(r=>r.method==="POST"&&r.url.endsWith("/administrators/search"))).toBe(true);
  assert.equal(state.requests.some(r=>r.url.includes("@")||r.url.includes("%40")),false);
  await page.getByRole("button",{name:"Clear filters",exact:true}).click();
  await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("table").getByRole("row")).toHaveCount(3);
  await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();
  await expect(page.getByRole("menuitem",{name:"View details",exact:true})).toBeVisible();
  await expect(page.getByRole("menuitem",{name:"Remove user",exact:true})).toHaveCount(0);
  await page.keyboard.press("Escape");await expect(page.getByRole("menu")).toHaveCount(0);
  await page.getByRole("button",{name:"Next",exact:true}).click();
  await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();
  await page.getByRole("menuitem",{name:"Disable user",exact:true}).click();
  const dialog=page.getByRole("alertdialog");
  await dialog.getByLabel("Reason",{exact:true}).fill("Synthetic disable");
  await dialog.getByRole("button",{name:"Disable administrator",exact:true}).click();
  await expect(dialog).toHaveCount(0);assert.equal(state.writes.at(-1).body.action,"DISABLE_IDENTITY");
  await page.getByRole("button",{name:"Next",exact:true}).click();
  await page.getByRole("button",{name:"Actions for support@example.test",exact:true}).click();
  await page.getByRole("menuitem",{name:"Remove user",exact:true}).click();
  await dialog.getByLabel("Reason",{exact:true}).fill("Synthetic removal");
  await dialog.getByRole("button",{name:"Remove user",exact:true}).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("button",{name:"Next",exact:true})).toBeDisabled();
  assert.equal(state.writes.at(-1).body.action,"REMOVE_IDENTITY");
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 }finally{await browser.close()}
});
test("Invite User includes names, validates, retains retry identity and accepts real terminal response metadata",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:390,height:844});try{
  await page.goto(origin+"/platform/users");await page.getByRole("button",{name:"Invite User",exact:true}).click();
  const dialog=page.getByRole("alertdialog");
  await expect(dialog.getByText(/Choose a Platform Admin role/)).toHaveCount(0);
  await dialog.getByLabel("First name",{exact:true}).fill("Amina");
  await dialog.getByLabel("Last name",{exact:true}).fill("Njoroge");
  await dialog.getByLabel("Email address",{exact:true}).fill("invalid");
  await dialog.getByLabel("Role",{exact:true}).selectOption("PLATFORM_OWNER");
  await expect(dialog.getByLabel("Reason",{exact:true})).toHaveCount(0);
  await expect(dialog.getByRole("button",{name:"Issue invitation",exact:true})).toBeDisabled();
  await dialog.getByLabel("Email address",{exact:true}).fill("new@example.test");
  state.invitationStatus=503;await dialog.getByRole("button",{name:"Issue invitation",exact:true}).click();
  await expect(page.locator("[data-sonner-toast]").filter({hasText:"unavailable"})).toBeVisible();
  await expect(dialog.getByRole("status")).toHaveCount(0);
  const key=state.writes[0].headers["idempotency-key"];state.invitationStatus=201;
  await dialog.getByRole("button",{name:"Issue invitation",exact:true}).click();
  await expect(page.locator("[data-sonner-toast]").filter({hasText:"accepted for delivery"})).toBeVisible();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("navigation",{name:"User views"}).getByRole("button",{name:"Invitations",exact:true})).toHaveAttribute("aria-pressed","true");
  await expect(page.getByRole("region",{name:"Platform administrators"})).toBeHidden();
  assert.equal(state.writes[1].headers["idempotency-key"],key);
  assert.equal(state.writes[1].body.first_name,"Amina");assert.equal(state.writes[1].body.last_name,"Njoroge");
  await expect(dialog.getByText(/outcome could not be confirmed/)).toHaveCount(0);
  assert.equal(state.writes[1].body.reason,"Administrator invitation");
 }finally{await browser.close()}
});
test("No data is shown in the table only for an actual empty result; denied reads remain distinguishable",{timeout:60000},async()=>{
 const{browser,page,state}=await open(undefined,true);try{
  await page.goto(origin+"/platform/users");await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("table")).toBeVisible();
  await expect(page.getByRole("button",{name:"Invite User",exact:true})).toHaveCount(0);
  await page.getByLabel("Email address",{exact:true}).fill("missing@example.test");
  await page.getByRole("button",{name:"Apply filters",exact:true}).click();
  await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("cell",{name:"No data found.",exact:true})).toBeVisible();
  state.denied=true;await page.reload();
  await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("table")).toBeVisible();
  await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("table").getByRole("alert")).toContainText("do not have access");
  await expect(page.getByRole("region",{name:"Platform administrators"}).getByRole("cell",{name:"No data found.",exact:true})).toHaveCount(0);
  assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
test("Security Audit cannot remove users or directly disable an Owner",{timeout:60000},async()=>{
 const{browser,page,state}=await open(undefined,false,["platform.admin.read","platform.admin.identity.disable","platform.admin.session.revoke","platform.governance.review"]);try{
  state.ownerCount=2;await page.goto(origin+"/platform/users");
  await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();
  await expect(page.getByRole("menuitem",{name:"View details",exact:true})).toBeVisible();
  await expect(page.getByRole("menuitem",{name:"Disable user",exact:true})).toHaveCount(0);
  await expect(page.getByRole("menuitem",{name:"Remove user",exact:true})).toHaveCount(0);
  assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});

test("Invited users appear with names and own-account revocation is never offered",{timeout:60000},async()=>{
 const{browser,page,state}=await open({width:1280,height:900});try{
  state.ownerCount=2;state.users[0].is_current_user=true;
  state.invitations=[{id:id(80),email:"amina@example.test",first_name:"Amina",last_name:"Njoroge",role_key:"PLATFORM_OWNER",state:"ACTIVE",version:1,created_at:at,expires_at:new Date(Date.now()+86400000).toISOString(),delivery_state:"ACCEPTED_FOR_DELIVERY",governance_change_id:null}];
  await page.goto(origin+"/platform/users");
  const invited=page.getByRole("region",{name:"Invited users"});
  await expect(invited).toBeHidden();
  await page.getByRole("navigation",{name:"User views"}).getByRole("button",{name:"Invitations",exact:true}).click();
  await expect(page.getByRole("region",{name:"Platform administrators"})).toBeHidden();
  await expect(invited.getByRole("row").filter({hasText:"amina@example.test"})).toContainText("Amina Njoroge");
  await expect(invited.getByRole("row").filter({hasText:"amina@example.test"})).toContainText("Invited");
  await page.getByRole("navigation",{name:"User views"}).getByRole("button",{name:"Users",exact:true}).click();
  await expect(invited).toBeHidden();
  await page.getByRole("button",{name:"Actions for owner@example.test",exact:true}).click();
  await expect(page.getByRole("menuitem",{name:"View details",exact:true})).toBeVisible();
  for(const name of ["Revoke sessions","Remove user","Disable user","Manage roles"])await expect(page.getByRole("menuitem",{name,exact:true})).toHaveCount(0);
  assert.equal(state.writes.length,0);
 }finally{await browser.close()}
});
