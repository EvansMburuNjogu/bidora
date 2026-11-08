import "server-only";
import { boundedUpstreamBody } from "@/lib/server/bounded-upstream-body";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

function failure(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "PLATFORM_DASHBOARD_UNAVAILABLE", message: "The financial overview is unavailable or the request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
}
export async function proxyPlatformDashboard(request: Request, kind: "dashboard" | "overview" | "references/organizations" | "references/support-assignees" = "dashboard"): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  const length = Number(request.headers.get("content-length") ?? "0");
  const query = new URL(request.url).searchParams;
  const reference = kind.startsWith("references/");
  if (reference && ([...query.keys()].some(key => key !== "search") || query.getAll("search").length > 1 || (query.get("search") ?? "").length > 100 || /[\u0000-\u001f]/.test(query.get("search") ?? ""))) return failure(400);
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation) || !Number.isFinite(length) || length !== 0 || request.body || !reference && new URL(request.url).search) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/${kind}${kind === "dashboard" ? "?limit=5" : reference && query.size ? `?${query}` : ""}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 400, 401, 403, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let body: ArrayBuffer;
  try { const bytes = await boundedUpstreamBody(upstream, 262_144); if (!bytes) return failure(503); body = bytes; } catch { return failure(503); }
  if (body.byteLength > 262_144) return failure(503);
  const responseHeaders = new Headers({ "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
