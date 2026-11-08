import assert from "node:assert/strict";
import { once } from "node:events";
import { rm } from "node:fs/promises";
import http from "node:http";
import net from "node:net";
import path from "node:path";
import { spawn } from "node:child_process";
import test from "node:test";

const root = path.resolve(import.meta.dirname, "../..");
const distDir = `.next-auth-test-${process.pid}`;
let backend;
let next;
let apiOrigin;
let webOrigin;
let forwarded = [];
let loginAuditContext = null;
let cancelAuditContext = null;
let cancelStatus = 204;
let googleRedirect = "valid";
let logs = "";
const platformRequests = [];
const managedRequests = [];
let platformStatus = 200;
let platformError = "PLATFORM_SETTING_NOT_FOUND";
let sessionMutation = null;
let bootstrapMounted = true;
let publicMinimum = 20;
const onboardingRequests=[];

async function freePort() {
  const server = net.createServer();
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const { port } = server.address();
  server.close();
  await once(server, "close");
  return port;
}

function jsonError(code) {
  return JSON.stringify({ error: { code, message: "The request was rejected.", field_errors: [], trace_id: crypto.randomUUID(), retryable: false } });
}

async function waitForNext(origin) {
  for (let attempt = 0; attempt < 120; attempt += 1) {
    try {
      const response = await fetch(origin + "/api/auth/not-an-operation", { redirect: "manual" });
      if (response.status === 405) return;
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`Next.js test server did not start:\n${logs.slice(-4000)}`);
}

test("public password requirements use current values and reject invalid policy responses", async () => {
  publicMinimum=20;let response=await fetch(webOrigin+"/api/auth/password-policy");assert.equal(response.status,200);assert.equal((await response.json()).data.minimum_password_bytes,20);
  publicMinimum=24;response=await fetch(webOrigin+"/api/auth/password-policy");assert.equal((await response.json()).data.minimum_password_bytes,24);
  publicMinimum=8;response=await fetch(webOrigin+"/api/auth/password-policy");assert.equal(response.status,503);publicMinimum=20;
});

test("Platform BFF paths preserve filters, context and safe read responses", async () => {
  platformStatus = 200;
  const cases = [
    ["/api/platform/dashboard", "/v1/platform/dashboard?limit=5"],
    ["/api/platform/tenants?search=Acme&country=KE&state=ACTIVE", "/v1/platform/tenants?search=Acme&country=KE&state=ACTIVE"],
    ["/api/platform/administrators", "/v1/platform/administrators"],
    ["/api/platform/administrator-changes?state=PENDING_APPROVAL", "/v1/platform/administrator-changes?state=PENDING_APPROVAL"],
    ["/api/platform/commercial/invoices?limit=10&state=REVIEW_REQUIRED", "/v1/platform/commercial/invoices?limit=10&state=REVIEW_REQUIRED"],
    ["/api/platform/ai-usage?limit=10", "/v1/platform/commercial/ai-usage?limit=10"],
    ["/api/platform/communications/attempts?limit=10", "/v1/platform/notifications/attempts?limit=10"],
    ["/api/platform/plans/management", "/v1/platform/commercial/plans/management"],
    ["/api/platform/audit/actions?limit=10&sort=ASC&search=User", "/v1/platform/audit/actions?limit=10&sort=ASC&search=User"],
    ["/api/platform/information", "/v1/platform/information"],
    ["/api/platform/integrations/openai/versions", "/v1/platform/integration-settings/OPENAI/versions?limit=25"],
    ["/api/platform/smtp/versions", "/v1/platform/integration-settings/SMTP/versions?limit=25"],
    ["/api/platform/settings/google", "/v1/platform/settings/google"],
    ["/api/platform/settings/google/history", "/v1/platform/settings/google/history"],
    ["/api/platform/notification-templates", "/v1/platform/notification-templates"],
    ["/api/platform/notification-templates/identity.password_recovery/history", "/v1/platform/notification-templates/identity.password_recovery/history"],
  ];
  for (const [path, upstream] of cases) {
    const response = await fetch(webOrigin + path, { headers: { "X-Bidora-Platform-Context-Generation": "7" } });
    assert.equal(response.status, 200, path);
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.deepEqual(platformRequests.at(-1), { path: upstream, method: "GET", generation: "7", origin: webOrigin });
  }
});

test("administrator invitation BFF forwards one guarded command and rejects missing authority before I/O", async () => {
  const key = "00000000-0000-4000-8000-000000000001";
  const headers = { Origin: webOrigin, "Content-Type": "application/json", "X-Bidora-Platform-Context-Generation": "7", "X-CSRF-Token": "A".repeat(43), "Idempotency-Key": key };
  const path = "/api/platform/administrator-invitations";
  const body = JSON.stringify({ email: "invited@example.test", role_key: "PLATFORM_OWNER", reason: "Operational coverage" });
  const before = platformRequests.length;
  const response = await fetch(webOrigin + path, { method: "POST", headers, body });
  assert.equal(response.status, 201);
  assert.equal(response.headers.get("cache-control"), "private, no-store");
  assert.deepEqual(platformRequests.at(-1), { path: "/v1/platform/administrator-invitations", method: "POST", generation: "7", origin: webOrigin });
  assert.deepEqual(managedRequests.at(-1), { version: undefined, key, csrf: headers["X-CSRF-Token"], body });
  for (const name of ["Origin", "X-CSRF-Token", "Idempotency-Key", "X-Bidora-Platform-Context-Generation"]) {
    const invalid = { ...headers }; delete invalid[name];
    assert.ok([400, 403].includes((await fetch(webOrigin + path, { method: "POST", headers: invalid, body })).status), name);
  }
  assert.equal((await fetch(webOrigin + path, { method: "POST", headers, body: JSON.stringify({ ...JSON.parse(body), secret: "unwanted" }) })).status, 400);
  assert.equal(platformRequests.length, before + 1);
});

test("financial adjustment BFF forwards only guarded maker/checker commands", async () => {
 const id="00000000-0000-4000-8000-000000000002",path="/api/platform/commercial/financial-adjustments";
 const headers={Origin:webOrigin,"Content-Type":"application/json","X-Bidora-Platform-Context-Generation":"7","X-CSRF-Token":"A".repeat(43),"Idempotency-Key":"00000000-0000-4000-8000-000000000001"};
 const proposal={invoice_id:id,action:"ISSUE_CREDIT",reason_code:"SERVICE_CREDIT",amount_minor_units:200,entitlement_disposition:"NO_ENTITLEMENT_CHANGE",proposal_reason:"Reviewed service credit",expected_invoice_version:3,expected_payment_version:4};
 const before=platformRequests.length;
 for(const invalid of [{...proposal,extra:true},{...proposal,action:"REQUEST_REFUND"},{...proposal,amount_minor_units:0}])assert.equal((await fetch(webOrigin+path,{method:"POST",headers,body:JSON.stringify(invalid)})).status,400);
 assert.equal((await fetch(webOrigin+path,{method:"POST",headers:{...headers,Origin:"https://invalid.test"},body:JSON.stringify(proposal)})).status,400);
 assert.equal(platformRequests.length,before);
 const created=await fetch(webOrigin+path,{method:"POST",headers,body:JSON.stringify(proposal)});assert.equal(created.status,201);assert.equal(created.headers.get("cache-control"),"private, no-store");
 assert.deepEqual(platformRequests.at(-1),{path:"/v1/platform/commercial/financial-adjustments",method:"POST",generation:"7",origin:webOrigin});
 assert.deepEqual(managedRequests.at(-1),{version:undefined,key:headers["Idempotency-Key"],csrf:headers["X-CSRF-Token"],body:JSON.stringify(proposal)});
 const decisionPath=`${path}/${id}/decision`;
 assert.equal((await fetch(webOrigin+`${path}/invalid/decision`,{method:"POST",headers,body:JSON.stringify({decision:"REJECTED",reason:"Not eligible",expected_version:1})})).status,400);
 assert.equal((await fetch(webOrigin+decisionPath,{method:"POST",headers,body:JSON.stringify({decision:"APPROVED",reason:"",expected_version:1})})).status,400);
 assert.equal((await fetch(webOrigin+decisionPath,{method:"POST",headers,body:JSON.stringify({decision:"REJECTED",reason:"Not eligible",expected_version:1})})).status,200);
 assert.deepEqual(platformRequests.at(-1),{path:`/v1/platform/commercial/financial-adjustments/${id}/decision`,method:"POST",generation:"7",origin:webOrigin});
});
test("billing reconciliation BFF forwards a bounded request without claiming settlement", async () => {
  const paymentID = "00000000-0000-4000-8000-000000000002";
  const path = `/api/platform/commercial/payments/${paymentID}/reconciliation`;
  const headers = { Origin: webOrigin, "Content-Type": "application/json", "X-Bidora-Platform-Context-Generation": "7", "X-CSRF-Token": "A".repeat(43), "Idempotency-Key": "00000000-0000-4000-8000-000000000001" };
  const body = JSON.stringify({ expected_version: 3, reason: "Verify a settled observation" });
  const before = platformRequests.length;
  const response = await fetch(webOrigin + path, { method: "POST", headers, body });
  assert.equal(response.status, 202);
  assert.equal(response.headers.get("cache-control"), "private, no-store");
  assert.deepEqual(platformRequests.at(-1), { path: `/v1/platform/commercial/payments/${paymentID}/reconciliation`, method: "POST", generation: "7", origin: webOrigin });
  assert.deepEqual(managedRequests.at(-1), { version: undefined, key: headers["Idempotency-Key"], csrf: headers["X-CSRF-Token"], body });
  for (const name of ["Origin", "X-CSRF-Token", "Idempotency-Key", "X-Bidora-Platform-Context-Generation"]) {
    const invalid = { ...headers }; delete invalid[name];
    assert.equal((await fetch(webOrigin + path, { method: "POST", headers: invalid, body })).status, 400, name);
  }
  assert.equal((await fetch(webOrigin + path, { method: "POST", headers, body: JSON.stringify({ expected_version: 3, reason: "Verify", extra: true }) })).status, 400);
  assert.equal(platformRequests.length, before + 1);
});

test("Platform plan BFF bounds versioned draft and publish commands", async () => {
  const headers = { Origin: webOrigin, "Content-Type": "application/json", "X-CSRF-Token": "A".repeat(43), "Idempotency-Key": "00000000-0000-4000-8000-000000000001", "X-Bidora-Platform-Context-Generation": "7" };
  const before = platformRequests.length;
  const draft = { expected_head_version: 1, monthly_price: { currency: "KES", amount: "4500.00" }, annual_price: { currency: "KES", amount: "45000.00" }, additional_licence_monthly_price: { currency: "KES", amount: "750.00" }, bid_workspaces: 3, included_licences: 3, storage_bytes: 10737418240, document_versions: 250, ai_credits: 5000, concurrent_ai_jobs: 2 };
  const path = "/api/platform/plans/BASIC/drafts";
  for (const body of [{ ...draft, extra: "secret" }, { ...draft, monthly_price: { currency: "USD", amount: "4500.00" } }]) assert.equal((await fetch(webOrigin + path, { method: "POST", headers, body: JSON.stringify(body) })).status, 400);
  assert.equal((await fetch(webOrigin + "/api/platform/plans/TRIAL/drafts", { method: "POST", headers, body: JSON.stringify(draft) })).status, 400);
  assert.equal(platformRequests.length, before);
  assert.equal((await fetch(webOrigin + path, { method: "POST", headers, body: JSON.stringify(draft) })).status, 201);
  assert.deepEqual(platformRequests.at(-1), { path: "/v1/platform/commercial/plans/BASIC/drafts", method: "POST", generation: "7", origin: webOrigin });
  assert.equal((await fetch(webOrigin + "/api/platform/plans/BASIC/publish", { method: "POST", headers, body: JSON.stringify({ version: 2, expected_head_version: 2 }) })).status, 201);
  assert.deepEqual(platformRequests.at(-1), { path: "/v1/platform/commercial/plans/BASIC/publish", method: "POST", generation: "7", origin: webOrigin });
});

test("Platform subscription BFF forwards only scoped, versioned boundary commands", async () => {
  const tenant = "00000000-0000-4000-8000-000000000003";
  const headers = { Origin: webOrigin, "Content-Type": "application/json", "X-CSRF-Token": "A".repeat(43), "Idempotency-Key": "00000000-0000-4000-8000-000000000001", "X-Bidora-Platform-Context-Generation": "7" };
  const base = `/api/platform/subscriptions/${tenant}`;
  const before = platformRequests.length;
  assert.equal((await fetch(webOrigin + "/api/platform/subscriptions/not-a-tenant", { headers })).status, 400);
  assert.equal((await fetch(webOrigin + base + "/cancellation", { method: "POST", headers, body: JSON.stringify({ expected_version: 1, secret: "bad" }) })).status, 400);
  for (const query of ["limit=51", "limit=20&limit=30", "cursor=not-uuid", "state=UNKNOWN", "sort=DESC"]) assert.equal((await fetch(webOrigin + `/api/platform/subscriptions?${query}`, { headers: { "X-Bidora-Platform-Context-Generation": "7" } })).status, 400);
  assert.equal(platformRequests.length, before);
  assert.equal((await fetch(webOrigin + `/api/platform/subscriptions?limit=20&state=ACTIVE&cursor=${tenant}`, { headers: { "X-Bidora-Platform-Context-Generation": "7" } })).status, 200);
  assert.deepEqual(platformRequests.at(-1), { path: `/v1/platform/commercial/subscriptions?limit=20&state=ACTIVE&cursor=${tenant}`, method: "GET", generation: "7", origin: webOrigin });
  assert.equal((await fetch(webOrigin + base, { headers: { "X-Bidora-Platform-Context-Generation": "7" } })).status, 200);
  assert.deepEqual(platformRequests.at(-1), { path: `/v1/platform/commercial/subscriptions/${tenant}`, method: "GET", generation: "7", origin: webOrigin });
  assert.equal((await fetch(webOrigin + base + "/cancellation", { method: "POST", headers, body: JSON.stringify({ expected_version: 1 }) })).status, 200);
  assert.deepEqual(platformRequests.at(-1), { path: `/v1/platform/commercial/subscriptions/${tenant}/cancellation`, method: "POST", generation: "7", origin: webOrigin });
});

test("fresh invitation reissue BFF requires an explicit bounded command", async () => {
  const id = "00000000-0000-4000-8000-000000000003";
  const headers = { Origin: webOrigin, "Content-Type": "application/json", "X-CSRF-Token": "A".repeat(43), "Idempotency-Key": "00000000-0000-4000-8000-000000000001", "X-Bidora-Platform-Context-Generation": "7" };
  const path = `/api/platform/administrator-invitations/${id}/reissue`;
  const before = platformRequests.length;
  for (const body of [{ expected_version: 1, reason: "Explicit replacement", token: "unsafe" }, { expected_version: 0, reason: "Explicit replacement" }]) assert.equal((await fetch(webOrigin + path, { method: "POST", headers, body: JSON.stringify(body) })).status, 400);
  assert.equal(platformRequests.length, before);
  assert.equal((await fetch(webOrigin + "/api/platform/administrator-invitations", { headers: { "X-Bidora-Platform-Context-Generation": "7" } })).status, 200);
  assert.deepEqual(platformRequests.at(-1), { path: "/v1/platform/administrator-invitations?limit=20", method: "GET", generation: "7", origin: webOrigin });
  assert.equal((await fetch(webOrigin + path, { method: "POST", headers, body: JSON.stringify({ expected_version: 1, reason: "Explicit replacement" }) })).status, 201);
  assert.deepEqual(platformRequests.at(-1), { path: `/v1/platform/administrator-invitations/${id}/reissue`, method: "POST", generation: "7", origin: webOrigin });
});

test("PA-23 BFF rejects invalid filters locally and distinguishes missing list from missing detail", async () => {
  const headers = { "X-Bidora-Platform-Context-Generation": "7" };
  const before = platformRequests.length;
  for (const query of ["cursor=" + "A".repeat(385), "limit=10&limit=20", "from=2026-02-30", "sort=invalid"]) {
    const response = await fetch(webOrigin + "/api/platform/audit/actions?" + query, { headers });
    assert.equal(response.status, 400);
  }
  assert.equal(platformRequests.length, before);
  platformStatus = 404;
  try {
    const list = await fetch(webOrigin + "/api/platform/audit/actions", { headers });
    assert.equal(list.status, 503);
    assert.equal((await list.json()).error.code, "AUDIT_UNAVAILABLE");
    const detail = await fetch(webOrigin + "/api/platform/audit/actions/00000000-0000-4000-8000-000000000001", { headers });
    assert.equal(detail.status, 404);
  } finally { platformStatus = 200; }
});

test("Support BFF validates bounded case commands and context before forwarding", async () => {
  const before = platformRequests.length;
  const path = "/api/platform/support/cases";
  const headers = { Origin: webOrigin, "Content-Type": "application/json", "X-CSRF-Token": "A".repeat(43), "Idempotency-Key": "00000000-0000-4000-8000-000000000001", "X-Bidora-Platform-Context-Generation": "7" };
  for (const body of [{ tenant_id: "not-uuid", title: "Title", description: "Description" }, { tenant_id: "00000000-0000-4000-8000-000000000001", title: "Title", description: "Description", secret: "bad" }]) {
    assert.equal((await fetch(webOrigin + path, { method: "POST", headers, body: JSON.stringify(body) })).status, 400);
  }
  assert.equal(platformRequests.length, before);
  const list = await fetch(webOrigin + path + "?limit=20", { headers: { "X-Bidora-Platform-Context-Generation": "7" } });
  assert.equal(list.status, 200);
  assert.deepEqual(platformRequests.at(-1), { path: "/v1/platform/support/cases?limit=20", method: "GET", generation: "7", origin: webOrigin });
  const create = await fetch(webOrigin + path, { method: "POST", headers, body: JSON.stringify({ tenant_id: "00000000-0000-4000-8000-000000000001", title: "Title", description: "Description" }) });
  assert.equal(create.status, 201);
  assert.deepEqual(platformRequests.at(-1), { path: "/v1/platform/support/cases", method: "POST", generation: "7", origin: webOrigin });
  assert.equal((await fetch(webOrigin + path + "/00000000-0000-4000-8000-000000000001/unknown", { method: "POST", headers, body: "{}" })).status, 400);
});

test("audit export BFF forwards only a bounded UTC window and accepts CSV only", async () => {
  const headers = { "X-Bidora-Platform-Context-Generation": "7" };
  const path = "/api/platform/audit/actions/export?from=2026-03-01&to=2026-03-10";
  const before = platformRequests.length;
  for (const query of ["from=2026-03-01", "from=2026-03-01&to=2026-04-02", "from=2026-03-01&to=2026-03-10&sort=ASC", "from=2026-03-01&to=2026-03-10&to=2026-03-11"]) {
    assert.equal((await fetch(webOrigin + "/api/platform/audit/actions/export?" + query, { headers })).status, 400);
  }
  assert.equal(platformRequests.length, before);
  const response = await fetch(webOrigin + path, { headers });
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("cache-control"), "private, no-store");
  assert.equal(response.headers.get("content-type"), "text/csv; charset=utf-8");
  assert.match(await response.text(), /^id,occurred_at_utc/);
  assert.deepEqual(platformRequests.at(-1), { path: "/v1/platform/audit/actions/export?from=2026-03-01&to=2026-03-10", method: "GET", generation: "7", origin: webOrigin });
});

test("SMTP BFF preserves concealed setting-not-found rather than claiming dependency failure", async () => {
  platformStatus = 404;
  try {
    const options = { method: "POST", headers: { Origin: webOrigin, "Content-Type": "application/json", "X-Bidora-Platform-Context-Generation": "7", "X-CSRF-Token": "A".repeat(43) }, body: JSON.stringify({ version_id: "00000000-0000-4000-8000-000000000001", version: 1, reason: "Verify configuration" }) };
    const missing = await fetch(webOrigin + "/api/platform/smtp/test", options);
    assert.equal(missing.status, 404);
    platformError = "NOT_FOUND";
    const unmounted = await fetch(webOrigin + "/api/platform/smtp/test", options);
    assert.equal(unmounted.status, 503);
  } finally { platformStatus = 200; platformError = "PLATFORM_SETTING_NOT_FOUND"; }
});

test("managed settings BFF forwards guarded commands and rejects missing authority headers before I/O", async () => {
  platformStatus = 200;
  const headers = { Origin: webOrigin, "Content-Type": "application/json", "X-Bidora-Platform-Context-Generation": "7", "X-CSRF-Token": "A".repeat(43), "If-Match": '"2"', "Idempotency-Key": "00000000-0000-4000-8000-000000000001" };
  for (const [path, method] of [["/api/platform/settings/google", "PUT"], ["/api/platform/settings/google/test", "POST"], ["/api/platform/notification-templates/identity.password_recovery", "PUT"], ["/api/platform/notification-templates/identity.password_recovery/test", "POST"]]) {
    const response = await fetch(webOrigin + path, { method, headers, body: "{}" });
    assert.equal(response.status, 200, path);
    assert.deepEqual(managedRequests.at(-1), { version: '"2"', key: headers["Idempotency-Key"], csrf: headers["X-CSRF-Token"], body: "{}" });
  }
  const count = platformRequests.length;
  for (const name of ["Origin", "X-CSRF-Token", "If-Match", "Idempotency-Key", "X-Bidora-Platform-Context-Generation"]) {
    const invalid = { ...headers }; delete invalid[name];
    const response = await fetch(webOrigin + "/api/platform/settings/google", { method: "PUT", headers: invalid, body: "{}" });
    assert.ok([400, 403].includes(response.status), name);
  }
  const oversized = await fetch(webOrigin + "/api/platform/settings/google", { method: "PUT", headers, body: JSON.stringify({ value: "x".repeat(17_000) }) });
  assert.equal(oversized.status, 400);
  assert.equal(platformRequests.length, count);
  const previewHeaders = { ...headers }; delete previewHeaders["If-Match"]; delete previewHeaders["Idempotency-Key"];
  assert.equal((await fetch(webOrigin + "/api/platform/notification-templates/identity.password_recovery/preview", { method: "POST", headers: previewHeaders, body: "{}" })).status, 200);
});

function assertSafeError(body) {
  assert.deepEqual(Object.keys(body.error).sort(), ["code", "field_errors", "message", "retryable", "trace_id"]);
  assert.match(body.error.trace_id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i);
}

async function postLogin(body, extraHeaders = {}) {
  return fetch(webOrigin + "/api/auth/login", {
    method: "POST",
    redirect: "manual",
    headers: { Origin: webOrigin, "Content-Type": "application/json", ...extraHeaders },
    body,
  });
}

test.before(async () => {
  const apiPort = await freePort();
  const webPort = await freePort();
  apiOrigin = `http://127.0.0.1:${apiPort}`;
  webOrigin = `http://127.0.0.1:${webPort}`;
  backend = http.createServer((request, response) => {
    forwarded.push(request.headers["x-forwarded-for"] ?? null);
    if(request.url.startsWith("/v1/auth/administrator-invitation/")||request.url==="/v1/platform/administrator-invitations/accept"){
      let body="";request.on("data",chunk=>body+=chunk);request.on("end",()=>{
        onboardingRequests.push({path:request.url,body:JSON.parse(body),headers:request.headers});
        if(request.url.endsWith("/preview")){response.writeHead(200,{"Content-Type":"application/json"});response.end(JSON.stringify({data:{email:"amina@example.test",first_name:"Amina",last_name:"Njoroge",role_key:"PLATFORM_OWNER",account_exists:false,expires_at:new Date(Date.now()+3600000).toISOString()}}));return}
        if(request.url.endsWith("/setup")){response.writeHead(202,{"Content-Type":"application/json","X-Bidora-Challenge-Expires-At":new Date(Date.now()+300000).toISOString(),"Set-Cookie":"__Host-bidora_login_code="+"B".repeat(43)+"; Path=/; Secure; HttpOnly; SameSite=Lax"});response.end(JSON.stringify({data:{mfa_required:true,expires_at:new Date(Date.now()+300000).toISOString()}}));return}
        response.writeHead(204);response.end();
      });return;
    }
    if (request.url === "/v1/auth/login-code/cancel") {
      cancelAuditContext = { ip: request.headers["x-forwarded-for"], browser: request.headers["user-agent"], cookie: request.headers.cookie, origin: request.headers.origin };
      const status = request.headers["x-forwarded-for"] ? cancelStatus : 503;
      if (status === 204) response.writeHead(204);
      else response.writeHead(status, { "Content-Type": "application/json" });
      response.end(status === 204 ? undefined : jsonError("AUDIT_CONTEXT_UNAVAILABLE"));
      return;
    }
    if (request.url === "/v1/auth/login-code/verify") {
      loginAuditContext = { ip: request.headers["x-forwarded-for"], browser: request.headers["user-agent"] };
      if(request.headers.cookie?.includes("__Host-bidora_login_code=onboarding-verified")){response.writeHead(204);response.end();return;}
    }
    if (request.url === "/v1/auth/password-policy") { response.writeHead(200, { "Content-Type": "application/json" }); response.end(JSON.stringify({data:{minimum_password_bytes:publicMinimum,maximum_password_bytes:128},meta:{trace_id:"synthetic-policy"}})); return; }
    if (request.url === "/v1/auth/google/start") {
      response.writeHead(303, { Location: googleRedirect === "valid" ? "https://accounts.google.com/o/oauth2/v2/auth?client_id=synthetic&state=opaque" : "https://attacker.invalid/oauth?state=opaque" });
      response.end();
      return;
    }
    if (request.url === "/v1/auth/session/bootstrap" && !bootstrapMounted) {
      response.writeHead(404, { "Content-Type": "application/json" });
      response.end(jsonError("NOT_FOUND"));
      return;
    }
    if (request.url === "/v1/auth/session/bootstrap" || request.url === "/v1/auth/sessions" && request.method === "GET") {
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify({ data: "x".repeat(70_000) }));
      return;
    }
    if (request.url?.startsWith("/v1/auth/sessions/") && request.method === "DELETE") {
      sessionMutation = { cookie: request.headers.cookie, csrf: request.headers["x-csrf-token"], key: request.headers["idempotency-key"] };
      response.writeHead(204, { "Set-Cookie": "__Host-bidora_session=; Path=/; Max-Age=0; Secure; HttpOnly; SameSite=Lax" });
      response.end();
      return;
    }
    if (request.url.startsWith("/v1/platform/")) {
      platformRequests.push({ path: request.url, method: request.method, generation: request.headers["x-bidora-platform-context-generation"], origin: request.headers.origin });
      const reconciliation = /^\/v1\/platform\/commercial\/payments\/[0-9a-f-]{36}\/reconciliation$/.test(request.url);
      const adjustment = request.url.startsWith("/v1/platform/commercial/financial-adjustments") && request.method === "POST";
      const managed = request.url.startsWith("/v1/platform/settings/google") || request.url.startsWith("/v1/platform/notification-templates") || request.url === "/v1/platform/administrator-invitations" || reconciliation || adjustment;
      let managedBody = ""; request.on("data", (chunk) => { if (managed) managedBody += chunk; });
      request.on("end", () => {
        if (managed) managedRequests.push({ version: request.headers["if-match"], key: request.headers["idempotency-key"], csrf: request.headers["x-csrf-token"], body: managedBody });
        const status = platformStatus === 200 ? reconciliation ? 202 : request.url === "/v1/platform/commercial/financial-adjustments" && request.method === "POST" ? 201 : request.url === "/v1/platform/administrator-invitations" || request.url === "/v1/platform/support/cases" && request.method === "POST" || request.url.startsWith("/v1/platform/commercial/plans/") && request.method === "POST" || request.url.endsWith("/reissue") && request.method === "POST" ? 201 : 200 : platformStatus;
        if (request.url.startsWith("/v1/platform/audit/actions/export?") && status === 200) {
          response.writeHead(200, { "Content-Type": "text/csv; charset=utf-8", "Cache-Control": "private, no-store" });
          response.end("id,occurred_at_utc,action,module,result,administrator_id,affected_record_id\n");
          return;
        }
        response.writeHead(status, { "Content-Type": "application/json", "Cache-Control": "private, no-store" });
        response.end(status === 202 ? JSON.stringify({ data: { payment_id: request.url.split("/")[5], version: 4, reconciliation_requested: true, reconciliation_state: "DUE" }, meta: { trace_id: crypto.randomUUID(), next_cursor: null } }) : status === 201 ? JSON.stringify({ data: { id: "00000000-0000-4000-8000-000000000001", email: "invited@example.test", role_key: "PLATFORM_OWNER", state: "ACTIVE", version: 1, created_at: "2026-10-18T12:00:00Z", expires_at: "2026-10-21T12:00:00Z", delivery_state: "ACCEPTED_FOR_DELIVERY", governance_change_id: null }, meta: { trace_id: crypto.randomUUID() } }) : status === 200 ? JSON.stringify({ data: [], meta: { trace_id: crypto.randomUUID(), next_cursor: null } }) : jsonError(platformError));
      });
      return;
    }
    let bytes = "";
    request.on("data", (chunk) => { bytes += chunk; });
    request.on("end", () => {
      if (bytes.includes("oversized@example.invalid")) {
        response.writeHead(401, { "Content-Type": "application/json" });
        response.end(JSON.stringify({ error: "x".repeat(70_000) }));
        return;
      }
      if (bytes.includes("nonjson@example.invalid")) {
        response.writeHead(500, { "Content-Type": "text/plain" });
        response.end("private upstream diagnostic");
        return;
      }
      if (bytes.includes("missingroute@example.invalid")) {
        response.writeHead(404, { "Content-Type": "application/json" });
        response.end(jsonError("NOT_FOUND"));
        return;
      }
      response.writeHead(401, { "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Request-ID": "synthetic-request" });
      response.end(jsonError("CREDENTIALS_INVALID"));
    });
  });
  backend.listen(apiPort, "127.0.0.1");
  await once(backend, "listening");

  next = spawn(process.execPath, [path.join(root, "node_modules/next/dist/bin/next"), "dev", "-H", "127.0.0.1", "-p", String(webPort)], {
    cwd: root,
    env: { ...process.env, BIDORA_API_ORIGIN: apiOrigin, BIDORA_WEB_ORIGIN: webOrigin, BIDORA_BFF_CLIENT_IP_MODE: "trusted-header", BIDORA_NEXT_TEST_DIST_DIR: distDir },
    stdio: ["ignore", "pipe", "pipe"],
  });
  next.stdout.on("data", (chunk) => { logs = (logs + chunk).slice(-8000); });
  next.stderr.on("data", (chunk) => { logs = (logs + chunk).slice(-8000); });
  await waitForNext(webOrigin);
});

test.after(async () => {
  if (next && next.exitCode === null) {
    next.kill("SIGTERM");
    await Promise.race([once(next, "exit"), new Promise((resolve) => setTimeout(resolve, 5000))]);
    if (next.exitCode === null) next.kill("SIGKILL");
  }
  if (backend) {
    backend.close();
    await once(backend, "close");
  }
  await rm(path.join(root, distDir), { recursive: true, force: true });
});

test("authentication BFF fails closed on missing and spoofed client addresses", async () => {
  forwarded = [];
  for (const header of [undefined, "192.0.2.10, 198.51.100.4", "not-an-ip"]) {
    const response = await postLogin('{"email":"person@example.invalid","password":"synthetic password"}', header ? { "X-Bidora-Client-IP": header } : {});
    assert.equal(response.status, 503);
    assertSafeError(await response.json());
  }
  assert.deepEqual(forwarded, []);
});

test("authentication BFF forwards exactly one validated address and safe upstream JSON", async () => {
  forwarded = [];
  const response = await postLogin('{"email":"person@example.invalid","password":"synthetic password"}', { "X-Bidora-Client-IP": "192.0.2.10" });
  assert.equal(response.status, 401);
  assertSafeError(await response.json());
  assert.deepEqual(forwarded, ["192.0.2.10"]);
  assert.equal(response.headers.get("cache-control"), "private, no-store");
});

test("email font is public cross-origin without widening API CORS", async () => {
  const response = await fetch(webOrigin + "/fonts/manrope-latin.woff2", { headers: { Origin: "https://mail.example.test" } });
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("access-control-allow-origin"), "*");
  assert.match(response.headers.get("content-type") ?? "", /font\/woff2/);
  assert.equal(Buffer.from(await response.arrayBuffer()).subarray(0, 4).toString(), "wOF2");
  assert.equal((await fetch(webOrigin + "/api/auth/not-an-operation")).headers.get("access-control-allow-origin"), null);
});

test("email-code completion forwards ingress IP and browser and rejects missing context", async () => {
  forwarded = []; loginAuditContext = null;
  const headers = { Origin: webOrigin, "Content-Type": "application/json", "User-Agent": "Mozilla/5.0 Firefox/123", "X-Forwarded-For": "198.51.100.99" };
  const url = webOrigin + "/api/auth/login-code";
  const body = JSON.stringify({ code: "123456", method: "EMAIL_OTP" });
  assert.equal((await fetch(url, { method: "POST", headers, body })).status, 503);
  assert.equal(loginAuditContext, null); assert.deepEqual(forwarded, []);
  assert.equal((await fetch(url, { method: "POST", headers: { ...headers, "X-Bidora-Client-IP": "192.0.2.20" }, body })).status, 401);
  assert.deepEqual(loginAuditContext, { ip: "192.0.2.20", browser: headers["User-Agent"] });
  assert.deepEqual(forwarded, ["192.0.2.20"]);
});

test("login-code cancellation forwards trusted audit context and clears only its challenge cookie", async () => {
  cancelAuditContext = null; cancelStatus = 204;
  const url = webOrigin + "/api/auth/login-code/cancel";
  const headers = { Origin: webOrigin, "X-Bidora-Client-IP": "192.0.2.24", "X-Forwarded-For": "198.51.100.99", "User-Agent": "Mozilla/5.0 Firefox/123", Cookie: "__Host-bidora_login_code=synthetic-challenge; __Host-bidora_session=synthetic-session" };
  assert.equal((await fetch(url, { method: "POST", headers: { ...headers, Origin: "https://foreign.invalid" } })).status, 403);
  assert.equal(cancelAuditContext, null);
  assert.equal((await fetch(url + "?unexpected=1", { method: "POST", headers })).status, 400);
  assert.equal((await fetch(url, { method: "POST", headers, body: "{}" })).status, 400);
  assert.equal(cancelAuditContext, null);
  const response = await fetch(url, { method: "POST", headers });
  assert.equal(response.status, 204);
  assert.deepEqual(cancelAuditContext, { ip: "192.0.2.24", browser: headers["User-Agent"], cookie: headers.Cookie, origin: webOrigin });
  assert.equal(response.headers.get("cache-control"), "private, no-store");
  const cookies = response.headers.getSetCookie();
  assert.equal(cookies.length, 1);
  assert.match(cookies[0], /^__Host-bidora_login_code=;/);
  assert.match(cookies[0], /Max-Age=0/);
  assert.match(cookies[0], /Secure/);
  assert.match(cookies[0], /HttpOnly/);
  assert.doesNotMatch(cookies[0], /__Host-bidora_session=/);
});

test("login-code cancellation remains idempotent without a challenge and fails closed on unavailable context", async () => {
  const url = webOrigin + "/api/auth/login-code/cancel";
  const headers = { Origin: webOrigin, "X-Bidora-Client-IP": "192.0.2.24" };
  cancelStatus = 204;
  assert.equal((await fetch(url, { method: "POST", headers })).status, 204);
  const missingContext = await fetch(url, { method: "POST", headers: { Origin: webOrigin, "X-Forwarded-For": "198.51.100.99" } });
  assert.equal(missingContext.status, 503);
  assert.equal(missingContext.headers.getSetCookie().length, 0);
  cancelStatus = 503;
  try {
    const unavailable = await fetch(url, { method: "POST", headers });
    assert.equal(unavailable.status, 503);
    assert.equal(unavailable.headers.getSetCookie().length, 0);
  } finally { cancelStatus = 204; }
});

test("authentication BFF maps an unexpectedly unmounted known route to unavailable", async () => {
  const response = await postLogin('{"email":"missingroute@example.invalid","password":"synthetic password"}', { "X-Bidora-Client-IP": "192.0.2.10" });
  assert.equal(response.status, 503);
  const body = await response.json();
  assert.equal(body.error.code, "AUTH_UNAVAILABLE");
  assertSafeError(body);
});

test("authentication and session BFFs reject oversized streamed upstream responses", async () => {
  const login = await postLogin('{"email":"oversized@example.invalid","password":"synthetic password"}', { "X-Bidora-Client-IP": "192.0.2.10" });
  assert.equal(login.status, 503);
  assertSafeError(await login.json());
  for (const path of ["/api/session/bootstrap", "/api/session/sessions"]) {
    const response = await fetch(webOrigin + path);
    assert.equal(response.status, 503, path);
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assertSafeError(await response.json());
  }
});

test("session BFF reports a missing backend route as unavailable", async () => {
  bootstrapMounted = false;
  try {
    const response = await fetch(webOrigin + "/api/session/bootstrap");
    assert.equal(response.status, 503);
    assert.equal((await response.json()).error.code, "AUTH_UNAVAILABLE");
  } finally { bootstrapMounted = true; }
});

test("session revocation BFF checks browser authority and forwards idempotency without a request body", async () => {
  const id = "00000000-0000-4000-8000-000000000001";
  const key = "00000000-0000-4000-8000-000000000002";
  const url = webOrigin + `/api/session/sessions/${id}`;
  const headers = { Origin: webOrigin, "X-CSRF-Token": "A".repeat(43), "Idempotency-Key": key, Cookie: "__Host-bidora_session=synthetic" };
  sessionMutation = null;
  assert.equal((await fetch(url, { method: "DELETE", headers: { ...headers, Origin: "https://invalid.test" } })).status, 403);
  assert.equal((await fetch(url, { method: "DELETE", headers: { ...headers, "X-CSRF-Token": "bad" } })).status, 403);
  assert.equal((await fetch(url, { method: "DELETE", headers: { ...headers, "Idempotency-Key": "bad" } })).status, 400);
  assert.equal(sessionMutation, null);
  const response = await fetch(url, { method: "DELETE", headers });
  assert.equal(response.status, 204);
  assert.equal(response.headers.get("cache-control"), "private, no-store");
  assert.deepEqual(sessionMutation, { cookie: headers.Cookie, csrf: headers["X-CSRF-Token"], key });
});

test("authentication BFF conceals non-JSON upstream diagnostics", async () => {
  const response = await postLogin('{"email":"nonjson@example.invalid","password":"synthetic password"}', { "X-Bidora-Client-IP": "2001:db8::10" });
  assert.equal(response.status, 503);
  const text = await response.text();
  assert.doesNotMatch(text, /private upstream diagnostic/);
  assertSafeError(JSON.parse(text));
});

test("authentication BFF returns canonical local validation failures", async () => {
  let response = await fetch(webOrigin + "/api/auth/login", { method: "POST", headers: { Origin: webOrigin, "Content-Type": "text/plain", "X-Bidora-Client-IP": "192.0.2.10" }, body: "{}" });
  assert.equal(response.status, 415);
  assertSafeError(await response.json());
  response = await postLogin(JSON.stringify({ value: "x".repeat(5000) }), { "X-Bidora-Client-IP": "192.0.2.10" });
  assert.equal(response.status, 413);
  assertSafeError(await response.json());
  response = await fetch(webOrigin + "/api/auth/not-an-operation", { method: "POST", headers: { Origin: webOrigin } });
  assert.equal(response.status, 404);
  assertSafeError(await response.json());
});

test("Google initiation accepts only the fixed authorization endpoint", async () => {
  googleRedirect = "valid";
  let response = await fetch(webOrigin + "/api/auth/google-start", { method: "POST", redirect: "manual", headers: { Origin: webOrigin } });
  assert.equal(response.status, 303);
  assert.match(response.headers.get("location"), /^https:\/\/accounts\.google\.com\/o\/oauth2\/v2\/auth\?/);
  googleRedirect = "invalid";
  response = await fetch(webOrigin + "/api/auth/google-start", { method: "POST", redirect: "manual", headers: { Origin: webOrigin } });
  assert.equal(response.status, 303);
  assert.equal(new URL(response.headers.get("location"), webOrigin).pathname, "/auth/google/failed");
});
test("Invitation BFF holds the emailed capability in an HttpOnly cookie and preserves MFA and CSRF boundaries",async()=>{
 const token="A".repeat(43),common={Origin:webOrigin,"Content-Type":"application/json","X-Bidora-Client-IP":"192.0.2.10"};
 const post=(operation,body,extra={})=>fetch(webOrigin+"/api/auth/administrator-invitation/"+operation,{method:"POST",headers:{...common,...extra},body:JSON.stringify(body)});
 let response=await post("preview",{token},{Origin:"https://attacker.invalid"});assert.equal(response.status,403);
 response=await post("preview",{token});assert.equal(response.status,200);const cookie=response.headers.getSetCookie().find(v=>/^(?:__Host-bidora_administrator_invite|bidora_dev_administrator_invite)=/.test(v));assert.ok(cookie);assert.match(cookie,/HttpOnly/i);assert.match(cookie,/SameSite=Lax/i);assert.equal(JSON.stringify(await response.json()).includes(token),false);
 const credential=cookie.split(";")[0];
 let verified=await fetch(webOrigin+"/api/auth/login-code",{method:"POST",headers:{...common,Cookie:"__Host-bidora_login_code=onboarding-verified; "+credential},body:JSON.stringify({code:"123456",method:"EMAIL_OTP"})});assert.equal(verified.status,204);assert.equal(verified.headers.get("x-bidora-invitation-pending"),"1");
 verified=await fetch(webOrigin+"/api/auth/login-code",{method:"POST",headers:{...common,Cookie:"__Host-bidora_login_code=onboarding-verified; bidora_dev_administrator_invite="+token},body:JSON.stringify({code:"123456",method:"EMAIL_OTP"})});assert.equal(verified.status,204);assert.equal(verified.headers.get("x-bidora-invitation-pending"),null);
 response=await post("preview",{},{Cookie:credential});assert.equal(response.status,200);assert.equal(onboardingRequests.at(-1).body.token,token);
 response=await post("setup",{password:"Synthetic-password-123456",first_name:"Amina",last_name:"Njoroge",token:"C".repeat(43)},{Cookie:credential});assert.equal(response.status,400);
 response=await post("setup",{password:"Synthetic-password-123456",first_name:"Amina",last_name:"Njoroge"},{Cookie:credential});assert.equal(response.status,202);assert.equal(onboardingRequests.at(-1).body.token,token);assert.equal(onboardingRequests.at(-1).headers.cookie,undefined);assert.ok(response.headers.get("x-bidora-challenge-expires-at"));const issued=response.headers.getSetCookie().join(";");assert.match(issued,/(?:bidora_dev_login_code|__Host-bidora_login_code)=/);assert.match(issued,/(?:bidora_dev_administrator_invite|__Host-bidora_administrator_invite)=;/);assert.doesNotMatch(issued,/(?:bidora_dev_session|__Host-bidora_session)=/);
 response=await post("accept",{},{Cookie:credential,"Idempotency-Key":crypto.randomUUID()});assert.equal(response.status,403);
 response=await post("accept",{},{Cookie:credential,"X-CSRF-Token":"C".repeat(43),"Idempotency-Key":crypto.randomUUID()});assert.equal(response.status,204);assert.equal(onboardingRequests.at(-1).body.token,token);assert.equal(onboardingRequests.at(-1).headers["x-csrf-token"],"C".repeat(43));
 response=await post("setup",{password:"Synthetic-password-123456",first_name:"Amina",last_name:"Njoroge"});assert.equal(response.status,400);
 response=await post("setup",{password:"Synthetic-password-123456"},{Cookie:"bidora_dev_administrator_invite="+token});assert.equal(response.status,400);
});
