import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { integrationTrustRoots } from "@/lib/server/integration-trust";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
import { PLATFORM_ROLES } from "@/lib/platform-contract";

function failure(status: 400 | 403 | 503): Response {
  return Response.json({ error: { code: status === 503 ? "PLATFORM_UNAVAILABLE" : status === 403 ? "BROWSER_REQUEST_FORBIDDEN" : "INVALID_REQUEST", message: status === 503 ? "Platform administration is temporarily unavailable." : status === 403 ? "The browser request was rejected." : "The request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } }, {
    status,
    headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" },
  });
}

export async function proxyPlatformAdministrators(request: Request, administratorID?: string, history = false): Promise<Response> {
  if (history && administratorID === undefined) return failure(400);
  if (administratorID !== undefined && !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(administratorID)) return failure(400);
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url);
  const declaredLength = Number(request.headers.get("content-length") ?? "0");
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (!Number.isFinite(declaredLength) || declaredLength > 0 || !generation || !/^[1-9]\d{0,18}$/.test(generation)) return failure(400);
  const entries = [...url.searchParams.entries()];
  if (administratorID && !history && entries.length > 0) return failure(400);
  if (history && (entries.length > 1 || entries.some(([key, value]) => key !== "cursor" || !/^[A-Za-z0-9_-]{1,512}$/.test(value)))) return failure(400);
  if (!administratorID && (entries.length > 3 || new Set(entries.map(([name]) => name)).size !== entries.length || entries.some(([key, value]) =>
    key === "cursor" ? !/^[A-Za-z0-9_-]{1,512}$/.test(value) : key === "state" ? !["ACTIVE", "DISABLED"].includes(value) :
      key === "role_key" ? !PLATFORM_ROLES.some((role) => role === value) : true))) return failure(400);

  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);

  let upstream: Response;
  try {
    const path = administratorID
      ? `/v1/platform/administrators/${administratorID}${history ? `/assignment-history${url.search}` : ""}`
      : `/v1/platform/administrators${url.search}`;
    upstream = await fetch(backend + path, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) });
  } catch {
    return failure(503);
  }
  if (upstream.status === 404 && !administratorID) {
    await upstream.body?.cancel();
    return failure(503);
  }
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  for (const name of ["content-type", "retry-after", "x-request-id"]) {
    const value = upstream.headers.get(name);
    if (value) responseHeaders.set(name, value);
  }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  const body = new Uint8Array(await upstream.arrayBuffer());
  if (body.byteLength > 262_144 || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(503);
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}

// Exact email search uses a CSRF-protected body, not an access-loggable query URL.
export async function proxyAdministratorEmailSearch(request: Request): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  const csrf = request.headers.get("x-csrf-token");
  const length = Number(request.headers.get("content-length"));
  if (request.headers.get("origin") !== browser) return failure(403);
  if (new URL(request.url).search || request.headers.get("content-type")?.split(";")[0] !== "application/json" ||
      !generation || !/^[1-9]\d{0,18}$/.test(generation) || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) ||
      !Number.isFinite(length) || length < 2 || length > 512) return failure(400);
  let body: ArrayBuffer;
  try { body = await request.arrayBuffer(); } catch { return failure(400); }
  if (body.byteLength < 2 || body.byteLength > 512) return failure(400);
  let value: unknown;
  try { value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)); } catch { return failure(400); }
  if (!value || typeof value !== "object" || Array.isArray(value)) return failure(400);
  const search = value as Record<string, unknown>;
  if (Object.keys(search).some((key) => !["email", "state", "role_key"].includes(key)) ||
      typeof search.email !== "string" || search.email.length < 3 || search.email.length > 254 ||
      search.state !== undefined && !["ACTIVE", "DISABLED"].includes(String(search.state)) ||
      search.role_key !== undefined && !PLATFORM_ROLES.some((role) => role === search.role_key)) return failure(400);
  const headers = new Headers({ Accept: "application/json", "Content-Type": "application/json", Origin: browser,
    "X-Bidora-Platform-Context-Generation": generation, "X-CSRF-Token": csrf });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/administrators/search`, { method: "POST", headers, body,
    cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 400, 401, 403, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") {
    await upstream.body?.cancel(); return failure(503);
  }
  let bytes: ArrayBuffer;
  try { bytes = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (bytes.byteLength > 65_536) return failure(503);
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const v = upstream.headers.get(name); if (v) responseHeaders.set(name, v); }
  for (const v of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(v, browser));
  return new Response(bytes, { status: upstream.status, headers: responseHeaders });
}

export async function proxyPlatformSessionRevocation(request: Request, administratorID: string): Promise<Response> {
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(administratorID)) return failure(400);
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return failure(503);
  if (request.headers.get("origin") !== browser) return failure(403);
  if (new URL(request.url).search || request.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(400);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  const csrf = request.headers.get("x-csrf-token");
  const key = request.headers.get("idempotency-key");
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation) || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !key || !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(key)) return failure(400);
  const length = Number(request.headers.get("content-length"));
  if (!Number.isFinite(length) || length < 2 || length > 4096) return failure(400);
  let bytes: ArrayBuffer;
  try { bytes = await request.arrayBuffer(); } catch { return failure(400); }
  if (bytes.byteLength < 2 || bytes.byteLength > 4096) return failure(400);
  const headers = new Headers({ Accept: "application/json", "Content-Type": "application/json", Origin: browser, "X-CSRF-Token": csrf, "X-Bidora-Platform-Context-Generation": generation, "Idempotency-Key": key });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try {
    upstream = await fetch(`${backend}/v1/platform/administrators/${administratorID}/sessions/revoke`, {
      method: "POST", headers, body: bytes, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000),
    });
  } catch { return failure(503); }
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  if (upstream.status === 204) {
    await upstream.body?.cancel();
    return new Response(null, { status: 204, headers: responseHeaders });
  }
  if (![400, 401, 403, 404, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") {
    await upstream.body?.cancel();
    return failure(503);
  }
  const responseBody = new Uint8Array(await upstream.arrayBuffer());
  if (responseBody.byteLength > 65_536) return failure(503);
  responseHeaders.set("Content-Type", "application/json");
  return new Response(responseBody, { status: upstream.status, headers: responseHeaders });
}

export function proxyPlatformSMTP(request: Request, operation: "active" | "create" | "test" | "propose" | "approve" | "reject"): Promise<Response> { return proxyPlatformIntegration(request,operation,"SMTP"); }

export async function proxyPlatformIntegration(request: Request, operation: "active" | "models" | "create" | "test" | "propose" | "approve" | "reject", kind: "SMTP" | "OPENAI" | "PESAPAL"): Promise<Response> {
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return failure(503);
  if (operation !== "active" && operation !== "models" && request.headers.get("origin") !== browser) return failure(403);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation)) return failure(400);
  let payload: Record<string, unknown> | null = null;
  if (operation !== "active" && operation !== "models") {
    const declaredLength = Number(request.headers.get("content-length") ?? "0");
    if (!Number.isFinite(declaredLength) || declaredLength < 2 || declaredLength > 1_100_000 || request.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(400);
    try { const reader=request.body?.getReader();if(!reader)return failure(400);const chunks:Uint8Array[]=[];let length=0;for(;;){const part=await reader.read();if(part.done)break;length+=part.value.byteLength;if(length>1_100_000){await reader.cancel();return failure(400);}chunks.push(part.value);}const all=new Uint8Array(length);let offset=0;for(const chunk of chunks){all.set(chunk,offset);offset+=chunk.byteLength;}const parsed:unknown=JSON.parse(new TextDecoder("utf-8",{fatal:true}).decode(all));if(!parsed||typeof parsed!=="object"||Array.isArray(parsed))return failure(400);payload=parsed as Record<string,unknown>; } catch { return failure(400); }
  }
  if (new URL(request.url).search) return failure(400);
  let path = `/v1/platform/integration-settings/${kind}`;
  if (operation === "models") path += "/models";
  let body: string | undefined;
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  if (operation === "create") {
    path += "/versions";
    body = JSON.stringify(payload);
  } else if (operation === "test" || operation === "propose") {
    const id = typeof payload?.version_id === "string" && /^[0-9a-f-]{36}$/i.test(payload.version_id) ? payload.version_id : null;
    const version = typeof payload?.version === "number" && Number.isSafeInteger(payload.version) && payload.version > 0 ? payload.version : null;
    if (!id || !version) return failure(400);
    path += `/versions/${id}/${operation === "test" ? "test" : "activation-proposals"}`;
    headers.set("If-Match", `\"${version}\"`);
    const { version_id: _id, version: _version, ...command } = payload!;
    void _id; void _version;
    body = JSON.stringify(command);
  } else if (operation === "approve" || operation === "reject") {
    const id = typeof payload?.change_id === "string" && /^[0-9a-f-]{36}$/i.test(payload.change_id) ? payload.change_id : null;
    const version = typeof payload?.version === "number" && Number.isSafeInteger(payload.version) && payload.version > 0 ? payload.version : null;
    if (!id || !version || typeof payload?.reason !== "string") return failure(400);
    path = `/v1/platform/integration-setting-changes/${id}/${operation}`;
    headers.set("If-Match", `\"${version}\"`);
    body = JSON.stringify({ reason: payload.reason });
  }
  const method = operation === "active" || operation === "models" ? "GET" : "POST";
  if (body !== undefined) {
    headers.set("Content-Type", "application/json");
    const csrf = request.headers.get("x-csrf-token");
    if (!csrf) return failure(400);
    headers.set("X-CSRF-Token", csrf);
    const supplied = request.headers.get("idempotency-key");
    if (supplied && !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(supplied)) return failure(400);
    headers.set("Idempotency-Key", supplied ?? crypto.randomUUID());
  }
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  if (operation === "create" && payload) {
    if (kind === "PESAPAL" && payload.enabled === true && !payload.notification_id) {
      const notification = process.env.BIDORA_PESAPAL_NOTIFICATION_ID;
      if (!notification || !/^[0-9a-f-]{36}$/i.test(notification)) return failure(503);
      payload.notification_id = notification;
    }
    const defaults = payload.use_default_trust === true;
    delete payload.use_default_trust;
    if (!payload.trust_roots_pem && (kind === "SMTP" || kind === "OPENAI" && defaults)) {
      try { payload.trust_roots_pem = await integrationTrustRoots(kind === "OPENAI" ? 12_000 : 24_576); } catch { return failure(503); }
    }
    body = JSON.stringify(payload);
  }
  let upstream: Response;
  try { upstream = await fetch(backend + path, { method, headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(operation === "test" ? kind === "OPENAI" ? 25_000 : 135_000 : 15_000) }); }
  catch { return failure(503); }
  if (![200, 201, 400, 401, 403, 404, 409, 413, 415, 503].includes(upstream.status) ||
      upstream.headers.get("content-type")?.split(";")[0] !== "application/json") {
    await upstream.body?.cancel(); return failure(503);
  }
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  for (const name of ["content-type", "retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  let responseBody: ArrayBuffer;
  try { responseBody = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (responseBody.byteLength > 262_144) return failure(503);
  if (upstream.status === 404) {
    try {
      const value: unknown = JSON.parse(new TextDecoder().decode(responseBody));
      if (!value || typeof value !== "object" || !("error" in value) ||
          !value.error || typeof value.error !== "object" || !("code" in value.error) ||
          value.error.code !== "PLATFORM_SETTING_NOT_FOUND") return failure(503);
    } catch { return failure(503); }
  }
  return new Response(responseBody, { status: upstream.status, headers: responseHeaders });
}
