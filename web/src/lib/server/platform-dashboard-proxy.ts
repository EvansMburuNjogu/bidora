import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

function failure(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "PLATFORM_DASHBOARD_UNAVAILABLE", message: "The financial overview is unavailable or the request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
}
export async function proxyPlatformDashboard(request: Request): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  const length = Number(request.headers.get("content-length") ?? "0");
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation) || !Number.isFinite(length) || length !== 0 || request.body || new URL(request.url).search) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/dashboard?limit=5`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 400, 401, 403, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let body: ArrayBuffer;
  try { body = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (body.byteLength > 262_144) return failure(503);
  const responseHeaders = new Headers({ "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
