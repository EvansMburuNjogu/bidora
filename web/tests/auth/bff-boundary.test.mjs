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
let googleRedirect = "valid";
let logs = "";
const platformRequests = [];
const managedRequests = [];
let platformStatus = 200;
let platformError = "PLATFORM_SETTING_NOT_FOUND";

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
    if (request.url === "/v1/auth/login-code/verify") loginAuditContext = { ip: request.headers["x-forwarded-for"], browser: request.headers["user-agent"] };
    if (request.url === "/v1/auth/google/start") {
      response.writeHead(303, { Location: googleRedirect === "valid" ? "https://accounts.google.com/o/oauth2/v2/auth?client_id=synthetic&state=opaque" : "https://attacker.invalid/oauth?state=opaque" });
      response.end();
      return;
    }
    if (request.url.startsWith("/v1/platform/")) {
      platformRequests.push({ path: request.url, method: request.method, generation: request.headers["x-bidora-platform-context-generation"], origin: request.headers.origin });
      const managed = request.url.startsWith("/v1/platform/settings/google") || request.url.startsWith("/v1/platform/notification-templates");
      let managedBody = ""; request.on("data", (chunk) => { if (managed) managedBody += chunk; });
      request.on("end", () => {
        if (managed) managedRequests.push({ version: request.headers["if-match"], key: request.headers["idempotency-key"], csrf: request.headers["x-csrf-token"], body: managedBody });
        response.writeHead(platformStatus, { "Content-Type": "application/json", "Cache-Control": "private, no-store" });
        response.end(platformStatus === 200 ? JSON.stringify({ data: [], meta: { trace_id: crypto.randomUUID(), next_cursor: null } }) : jsonError(platformError));
      });
      return;
    }
    let bytes = "";
    request.on("data", (chunk) => { bytes += chunk; });
    request.on("end", () => {
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

test("authentication BFF maps an unexpectedly unmounted known route to unavailable", async () => {
  const response = await postLogin('{"email":"missingroute@example.invalid","password":"synthetic password"}', { "X-Bidora-Client-IP": "192.0.2.10" });
  assert.equal(response.status, 503);
  const body = await response.json();
  assert.equal(body.error.code, "AUTH_UNAVAILABLE");
  assertSafeError(body);
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
