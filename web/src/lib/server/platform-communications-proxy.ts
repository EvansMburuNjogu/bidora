import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

const responseBase = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" };
function failure(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "HISTORY_UNAVAILABLE", message: "Notification history is unavailable or the request is invalid." } }, { status, headers: responseBase });
}
export async function proxyPlatformCommunications(request: Request): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url);
  const params = [...url.searchParams.entries()];
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (request.body || request.headers.get("content-length") && request.headers.get("content-length") !== "0" ||
    !generation || !/^[1-9]\d{0,18}$/.test(generation) || params.length > 6 ||
    new Set(params.map(([key]) => key)).size !== params.length || params.some(([key, value]) =>
      key === "limit" ? !/^(?:[1-9]|[1-9]\d|100)$/.test(value) : key === "cursor" ? !/^[A-Za-z0-9_-]{1,384}$/.test(value) :
      key === "category" ? !/^[A-Z][A-Z0-9_]{0,79}$/.test(value) : key === "status" ? !["NOT_ATTEMPTED","PENDING","SMTP_ACCEPTED","FAILED","AMBIGUOUS"].includes(value) :
      key === "from" || key === "to" ? !/^\d{4}-\d\d-\d\d$/.test(value) : true)) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/notifications/attempts${url.search}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 400, 401, 403, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let body: ArrayBuffer;
  try { body = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (body.byteLength > 4_194_304) return failure(503);
  const responseHeaders = new Headers({ ...responseBase, "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
