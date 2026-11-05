import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
function failure(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "PLATFORM_SETTINGS_UNAVAILABLE", message: "The integration setting is unavailable or the request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
}
export async function proxyIntegrationRead(request: Request, kind: "SMTP" | "OPENAI" | "PESAPAL", changeID?: string, active = false): Promise<Response> {
  if (changeID !== undefined && !uuid.test(changeID)) return failure(400);
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  const length = Number(request.headers.get("content-length") ?? "0");
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation) || !Number.isFinite(length) || length !== 0 || request.body) return failure(400);
  const entries = [...url.searchParams.entries()];
  if (changeID || active ? entries.length !== 0 : entries.length > 1 || entries.some(([name, value]) => name !== "cursor" || !/^[A-Za-z0-9_-]{1,256}$/.test(value))) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  const path = changeID ? `/v1/platform/integration-setting-changes/${changeID}` : active ? `/v1/platform/integration-settings/${kind}` : `/v1/platform/integration-settings/${kind}/versions?limit=25${url.search ? `&${url.search.slice(1)}` : ""}`;
  let upstream: Response;
  try { upstream = await fetch(backend + path, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 400, 401, 403, 404, 409, 503].includes(upstream.status) || upstream.status === 404 && !changeID && !active ||
      upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let body: ArrayBuffer;
  try { body = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (body.byteLength > 262_144) return failure(503);
  const responseHeaders = new Headers({ "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
export function proxySMTPRead(request: Request, changeID?: string): Promise<Response> { return proxyIntegrationRead(request, "SMTP", changeID); }
