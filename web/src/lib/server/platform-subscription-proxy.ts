import "server-only";
import { boundedUpstreamBody } from "@/lib/server/bounded-upstream-body";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
const uuidPattern = /^[a-f\d]{8}-[a-f\d]{4}-[1-8][a-f\d]{3}-[89ab][a-f\d]{3}-[a-f\d]{12}$/i;
function failed(status: 400 | 503) { return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "SUBSCRIPTION_UNAVAILABLE", message: "The request is invalid or subscription data is unavailable.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } }, { status, headers: { "Cache-Control": "private, no-store", Vary: "Cookie, Origin" } }); }
const states = ["TRIAL_ACTIVE", "PAYMENT_PENDING", "ACTIVE", "PAST_DUE", "SUSPENDED", "ENDED", "REVIEW_REQUIRED"];
export async function proxyPlatformSubscriptionInventory(request: Request): Promise<Response> {
  const browser=webOrigin(),backend=apiOrigin(),generation=request.headers.get("x-bidora-platform-context-generation");
  if(!browser||!backend)return failed(503);
  const url=new URL(request.url),entries=[...url.searchParams.entries()];
  if(request.method!=="GET"||request.body!==null||!generation||!/^[1-9]\d{0,18}$/.test(generation)||entries.length>3||new Set(entries.map(([name])=>name)).size!==entries.length||entries.some(([name,value])=>name==="limit"?!/^(?:[1-9]|[1-4]\d|50)$/.test(value):name==="cursor"?!uuidPattern.test(value):name==="state"?!states.includes(value):true))return failed(400);
  const headers=new Headers({Accept:"application/json",Origin:browser,"X-Bidora-Platform-Context-Generation":generation});
  forwardAuditContext(request, headers);
  const cookie=backendCookieHeader(request.headers.get("cookie"),browser);if(cookie)headers.set("Cookie",cookie);
  let upstream:Response;try{upstream=await fetch(`${backend}/v1/platform/commercial/subscriptions${url.search}`,{method:"GET",headers,cache:"no-store",redirect:"manual",signal:AbortSignal.timeout(10_000)})}catch{return failed(503)}
  if(![200,400,401,403,409,503].includes(upstream.status)||upstream.headers.get("content-type")?.split(";")[0]!=="application/json"){await upstream.body?.cancel();return failed(503)}
  let output:ArrayBuffer;try{const bytes=await boundedUpstreamBody(upstream);if(!bytes)return failed(503);output=bytes}catch{return failed(503)}
  if(output.byteLength>65_536)return failed(503);
  const responseHeaders=new Headers({"Content-Type":"application/json","Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff",Vary:"Cookie, Origin"});
  for(const name of ["retry-after","x-request-id"]){const value=upstream.headers.get(name);if(value)responseHeaders.set(name,value)}
  for(const value of upstream.headers.getSetCookie())responseHeaders.append("Set-Cookie",browserSetCookie(value,browser));
  return new Response(output,{status:upstream.status,headers:responseHeaders});
}
export async function proxyPlatformSubscription(request: Request, tenant: string, operation: "read" | "schedule" | "withdraw" | "suspend" | "reactivate"): Promise<Response> {
  const browser = webOrigin(), backend = apiOrigin(), generation = request.headers.get("x-bidora-platform-context-generation");
  if (!browser || !backend) return failed(503);
  if (!uuidPattern.test(tenant) || new URL(request.url).search || !generation || !/^[1-9]\d{0,18}$/.test(generation) || request.method !== (operation === "read" ? "GET" : "POST")) return failed(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  let body: ArrayBuffer | undefined;
  if (operation === "read") { if (request.body !== null) return failed(400); }
  else {
    const csrf = request.headers.get("x-csrf-token"), key = request.headers.get("idempotency-key");
    if (request.headers.get("origin") !== browser || request.headers.get("content-type")?.split(";")[0] !== "application/json" || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !key || !uuidPattern.test(key) || Number(request.headers.get("content-length") ?? "0") > 4096) return failed(400);
    try { const bytes = await boundedUpstreamBody(request, 4096); if (!bytes) return failed(400); body = bytes; if (body.byteLength < 2 || body.byteLength > 4096) return failed(400); const value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)); if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).length !== (operation === "suspend" || operation === "reactivate" ? 2 : 1) || ((operation === "suspend" || operation === "reactivate") && (typeof value.reason !== "string" || !value.reason.trim() || value.reason !== value.reason.trim() || new TextEncoder().encode(value.reason).byteLength > 2000)) || !Number.isSafeInteger(value.expected_version) || value.expected_version < 1) return failed(400); } catch { return failed(400); }
    headers.set("Content-Type", "application/json"); headers.set("X-CSRF-Token", csrf); headers.set("Idempotency-Key", key);
  }
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser); if (cookie) headers.set("Cookie", cookie);
  const tenantCommand = operation === "suspend" || operation === "reactivate";
  const path = operation === "read" ? "" : operation === "schedule" ? "/cancellation" : operation === "withdraw" ? "/cancellation/withdraw" : `/${operation}`;
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/${tenantCommand ? "tenants" : "commercial/subscriptions"}/${tenant}${path}`, { method: request.method, headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(15_000) }); } catch { return failed(503); }
  if (![200, 400, 401, 403, 404, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failed(503); }
  let output: ArrayBuffer; try { const bytes = await boundedUpstreamBody(upstream); if (!bytes) return failed(503); output = bytes; } catch { return failed(503); }
  if (output.byteLength > 65_536) return failed(503);
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(output, { status: upstream.status, headers: responseHeaders });
}
