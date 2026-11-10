import assert from "node:assert/strict";
import test from "node:test";
import { chromium, expect } from "@playwright/test";

const origin = process.env.BIDORA_WEB_URL ?? "http://127.0.0.1:3000";
if (!/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(origin)) throw Error("Use loopback; all APIs are intercepted.");
const code = "A".repeat(43), csrf = "B".repeat(42) + "A";
for (const kind of ["platform", "tenant"]) test(`${kind} invitation link prefills privately and only accepts on confirmation`, {timeout:60000}, async () => {
  const browser = await chromium.launch({headless:true});
  try {
    const page = await browser.newPage({viewport:{width:390,height:844}});
    const writes = [], urls = [];
    let status = 503, bootstrapStatus = 200, accountType = "UNASSIGNED";
    page.on("request", request => urls.push(request.url()));
    await page.route("**/api/**", async route => {
      const request = route.request(), path = new URL(request.url()).pathname;
      if (path === "/api/auth/invitation-details") return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{email:"i***@e***.test",role_key:"PLATFORM_OWNER",setup_state:"CREATE"}})});
      if (path === "/api/auth/password-policy") return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{minimum_password_bytes:12,maximum_password_bytes:128}})});
      if (path === "/api/session/bootstrap") return route.fulfill({status:bootstrapStatus,contentType:"application/json",body:JSON.stringify({data:{csrf_token:csrf}})});
      if (path === "/api/session/me") return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{account_type:accountType,portal:accountType==="PLATFORM_ADMIN"?{key:"PLATFORM_ADMIN",path:"/platform"}:null,platform_context_generation:accountType==="PLATFORM_ADMIN"?1:null,capabilities:accountType==="PLATFORM_ADMIN"?["platform.admin.read"]:[],required_actions:accountType==="PLATFORM_ADMIN"?[]:["SELECT_ACCOUNT_TYPE"],session:{assurance_level:"AAL1",idle_expires_at:new Date(Date.now()+3600000).toISOString(),absolute_expires_at:new Date(Date.now()+3600000).toISOString()}},meta:{trace_id:"fixture"}})});
      if (path === `/api/invitations/${kind}/accept`) {
        writes.push({body:request.postDataJSON(),headers:request.headers()});
        return route.fulfill(status===204?{status,body:""}:{status,contentType:"application/json",body:JSON.stringify({error:{code:"UNAVAILABLE"}})});
      }
      return route.fulfill({status:401,contentType:"application/json",body:"{}"});
    });
    await page.goto(`${origin}/invitations/${kind}#code=${code}`);
    await expect(page.getByLabel("Invitation code")).toHaveValue(code);
    await expect(page).toHaveURL(`${origin}/invitations/${kind}`);
    assert.equal(writes.length,0);
    assert.ok(urls.every(url => !url.includes(code)), "credential must not enter request URLs");
    bootstrapStatus=401;
    await page.getByRole("button",{name:"Accept invitation",exact:true}).click();
    await expect(page.locator("form").getByRole("alert")).toContainText("Sign in with the invited email");
    assert.equal(writes.length,0);
    bootstrapStatus=200;
    if (kind === "platform") {
      accountType="PLATFORM_ADMIN";
      await page.getByRole("button",{name:"Accept invitation",exact:true}).click();
      await expect(page.getByLabel("Confirm password",{exact:true})).toBeVisible();
      assert.equal(writes.length,0);
      accountType="UNASSIGNED";
      await page.goto(`${origin}/invitations/${kind}#code=${code}`);
      await expect(page.getByLabel("Invitation code")).toHaveValue(code);
    }
    await page.getByRole("button",{name:"Accept invitation",exact:true}).click();
    await expect(page.locator("form").getByRole("alert")).toContainText("couldn’t confirm");
    status=204;
    await page.getByRole("button",{name:"Accept invitation",exact:true}).click();
    await expect(page.getByRole("status")).toContainText("Invitation accepted.");
    assert.equal(writes.length,2);
    assert.deepEqual(writes[0].body,{token:code});
    assert.equal(writes[0].headers["idempotency-key"],writes[1].headers["idempotency-key"]);
    assert.equal(writes[0].headers["x-csrf-token"],csrf);
    assert.equal(await page.evaluate(() => location.hash),"");
    assert.ok(urls.every(url => !url.includes(code)));
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth<=innerWidth));
    await page.setViewportSize({width:1280,height:900});
    await expect(page.getByRole("status").filter({hasText:"Invitation accepted."})).toBeVisible();
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth<=innerWidth));
  } finally { await browser.close(); }
});

test("first-time invitation prompts for password and requires confirmed sign-out before registration", {timeout:60000}, async () => {
  const browser=await chromium.launch({headless:true});
  try {
    const page=await browser.newPage();
    let logoutStatus=403;
    const logouts=[], accepts=[], registrations=[];
    await page.route("**/api/**",async route=>{
      const request=route.request(), path=new URL(request.url()).pathname;
      if(path==="/api/auth/password-policy")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{minimum_password_bytes:12,maximum_password_bytes:128}})});
      if(path==="/api/auth/invitation-details")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{email:"f***@e***.test",role_key:"PLATFORM_OWNER"}})});
      if(path==="/api/auth/invitation-register"){registrations.push(request.postDataJSON());return route.fulfill({status:202,headers:{"X-Bidora-Code-Challenge":"00000000-0000-4000-8000-000000000001"},body:""});}
      if(path==="/api/session/bootstrap")return route.fulfill({status:200,contentType:"application/json",body:JSON.stringify({data:{csrf_token:csrf}})});
      if(path==="/api/auth/logout"){
        logouts.push(request.headers());
        return route.fulfill(logoutStatus===204?{status:204,body:""}:{status:logoutStatus,contentType:"application/json",body:"{}"});
      }
      if(path.includes("/invitations/")&&request.method()==="POST")accepts.push(path);
      return route.fulfill({status:401,contentType:"application/json",body:"{}"});
    });
    await page.goto(`${origin}/invitations/platform#code=${code}`);
    await expect(page.getByLabel("Confirm password",{exact:true})).toBeVisible();
    await expect(page.getByLabel("Password",{exact:true})).toBeVisible();
    await expect(page.getByLabel("Confirm password",{exact:true})).toBeVisible();
    assert.equal(logouts.length,0);
    await expect(page.getByLabel("First name",{exact:true})).toHaveCount(0);
    await expect(page.getByLabel("Last name",{exact:true})).toHaveCount(0);
    await expect(page.getByLabel("Email address",{exact:true})).toHaveValue("f***@e***.test");
    await expect(page.getByLabel("Email address",{exact:true})).toHaveAttribute("readonly","");
    await expect(page.getByLabel("Email address",{exact:true})).toBeDisabled();
    await expect(page.getByLabel("User type",{exact:true})).toBeDisabled();
    await expect(page.getByLabel("User type",{exact:true})).toHaveValue("Platform Owner");
    await expect(page.getByLabel("Password",{exact:true})).toHaveAttribute("placeholder","Enter your password");
    await expect(page.getByLabel("Confirm password",{exact:true})).toHaveAttribute("placeholder","Re-enter your password");
    await page.getByLabel("Password",{exact:true}).fill("Synthetic-invite-password-34!");
    await page.getByLabel("Confirm password",{exact:true}).fill("Synthetic-invite-password-34!");
    await page.getByRole("button",{name:"Set password",exact:true}).click();
    await expect(page.getByText("We couldn’t sign you out. Please try again before switching accounts.",{exact:true})).toBeVisible();
    assert.equal(registrations.length,0);
    await expect(page).toHaveURL(`${origin}/invitations/platform`);
    assert.equal(logouts[0]["x-csrf-token"],csrf);
    logoutStatus=204;
    await page.getByRole("button",{name:"Set password",exact:true}).click();
    await expect(page).toHaveURL(`${origin}/verify-email?challenge_ref=00000000-0000-4000-8000-000000000001`);
    assert.equal(registrations.length,1);
    assert.deepEqual(registrations[0],{token:code,password:"Synthetic-invite-password-34!"});
    assert.equal(logouts.length,2);
    assert.equal(accepts.length,0);
  } finally {await browser.close();}
});
