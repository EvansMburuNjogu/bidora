import assert from "node:assert/strict";
import { chromium,expect } from "@playwright/test";
const origin=process.env.BIDORA_WEB_URL??"http://127.0.0.1:3037";
if(!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin))throw Error("Use loopback origin.");
const id="11111111-1111-4111-8111-111111111111",at="2030-01-10T12:00:00Z",envelope=data=>({data,meta:{trace_id:id}});
const rows=[{recipient:"recipient@example.test",subject:"Reset your password",message:"Use [code redacted] to reset. [link redacted]",category:"PASSWORD_RECOVERY",attempt_number:1,disposition:"SMTP_ACCEPTED",failure_code:null,started_at:at,finished_at:at},{recipient:null,subject:null,message:null,category:"EMAIL_VERIFICATION",attempt_number:0,disposition:null,failure_code:null,started_at:at,finished_at:null}];
const browser=await chromium.launch({headless:true});
try{
 const page=await browser.newPage({viewport:{width:1280,height:900}});const queries=[];let denied=false;
 await page.route("**/api/**",async route=>{
  const u=new URL(route.request().url());let data,status=200;
  if(u.pathname==="/api/session/bootstrap")data=envelope({csrf_token:"A".repeat(43)});
  else if(u.pathname==="/api/session/me")data=envelope({account_type:"PLATFORM_ADMIN",portal:{key:"PLATFORM_ADMIN",path:"/platform"},platform_context_generation:1,capabilities:["platform.admin.read","platform.integration.failure.read"],session:{assurance_level:"AAL2",idle_expires_at:new Date(Date.now()+600000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()},required_actions:[]});
  else if(u.pathname==="/api/session/availability")data=envelope({maintenance_active:false,maintenance_message:""});
  else if(u.pathname==="/api/platform/communications/attempts"){
   queries.push(u.searchParams);status=denied?403:200;
   data=denied?{error:{code:"PLATFORM_FORBIDDEN"}}:{data:u.searchParams.has("category")?[rows[0]]:rows,meta:{trace_id:id,next_cursor:u.searchParams.has("category")?null:"c2FmZQ"}};
  }else return route.fulfill({status:503,contentType:"application/json",body:"{}"});
  return route.fulfill({status,contentType:"application/json",body:JSON.stringify(data)});
 });
 await page.goto(origin+"/platform/communications");
 await expect(page.getByRole("cell",{name:"recipient@example.test",exact:true})).toBeVisible();
 await page.getByText("View message",{exact:true}).click();
 await expect(page.getByText("Use [code redacted] to reset. [link redacted]",{exact:true})).toBeVisible();
 await expect(page.getByText("Message details were not recorded or are no longer retained.",{exact:true})).toBeVisible();
 await page.getByRole("button",{name:"Next",exact:true}).click();
 await expect.poll(()=>queries.at(-1)?.get("cursor")).toBe("c2FmZQ");
 await page.getByLabel("Category",{exact:true}).selectOption("PASSWORD_RECOVERY");
 await page.getByLabel("Outcome",{exact:true}).selectOption("SMTP_ACCEPTED");
 await page.getByLabel("From date (UTC)",{exact:true}).fill("2030-01-01");
 await page.getByLabel("To date (UTC)",{exact:true}).fill("2030-01-31");
 await page.getByRole("button",{name:"Apply filters",exact:true}).click();
 await expect.poll(()=>queries.at(-1)?.get("category")).toBe("PASSWORD_RECOVERY");
 const q=queries.at(-1);assert.equal(q.get("status"),"SMTP_ACCEPTED");assert.equal(q.get("from"),"2030-01-01");assert.equal(q.get("to"),"2030-01-31");assert.equal(q.has("cursor"),false);
 await page.getByLabel("From date (UTC)",{exact:true}).fill("2031-01-01");
 await page.getByRole("button",{name:"Apply filters",exact:true}).click();
 await expect(page.getByText("From date must be on or before To date.",{exact:true})).toBeVisible();
 await page.getByRole("button",{name:"Clear filters",exact:true}).click();
 await expect.poll(()=>queries.at(-1)?.has("category")).toBe(false);
 await page.setViewportSize({width:390,height:844});
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),true);
 denied=true;await page.getByRole("button",{name:"Refresh",exact:true}).click();
 await expect(page.getByText("Only authorised Platform Administrators with recent verification can view notification records.",{exact:true})).toBeVisible();
 await expect(page.getByRole("cell",{name:"recipient@example.test",exact:true})).toHaveCount(0);
 console.log("PASS: recipient/message display, historical gaps, category/outcome/UTC dates, cursor reset, validation/clear, mobile layout and denied access (intercepted APIs)");
}finally{await browser.close();}
