import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
import { platformTenantStates } from "@/lib/platform-tenant-contract";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
function failure(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "PLATFORM_UNAVAILABLE", message: "The tenant directory is unavailable or the request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
}
export async function proxyPlatformTenants(request: Request, tenantID?: string): Promise<Response> {
  if (tenantID !== undefined && !uuid.test(tenantID)) return failure(400);
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url);
  const length = Number(request.headers.get("content-length") ?? "0");
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (!Number.isFinite(length) || length !== 0 || request.body || !generation || !/^[1-9]\d{0,18}$/.test(generation)) return failure(400);
  const entries = [...url.searchParams.entries()];
  if (tenantID ? entries.length !== 0 : entries.length > 4 || entries.some(([key, value]) =>
      key === "cursor" ? !/^[A-Za-z0-9_-]{1,512}$/.test(value) : key === "state" ? !platformTenantStates.includes(value as (typeof platformTenantStates)[number]) : key === "country" ? !/^[A-Z]{2}$/.test(value) : key === "search" ? new TextEncoder().encode(value).length < 2 || new TextEncoder().encode(value).length > 100 || value.trim() !== value || /[\u0000-\u001f\u007f]/.test(value) : true) ||
      new Set(entries.map(([key]) => key)).size !== entries.length) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try {
    upstream = await fetch(backend + (tenantID ? `/v1/platform/tenants/${tenantID}` : `/v1/platform/tenants${url.search}`),
      { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) });
  } catch { return failure(503); }
  if (![200, 400, 401, 403, 404, 409, 503].includes(upstream.status) || upstream.status === 404 && !tenantID ||
      upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  const body = new Uint8Array(await upstream.arrayBuffer());
  if (body.byteLength > 262_144) return failure(503);
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
