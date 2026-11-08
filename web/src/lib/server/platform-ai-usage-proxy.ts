import "server-only";
import { boundedUpstreamBody } from "@/lib/server/bounded-upstream-body";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

const headersBase = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" };
function unavailable(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "USAGE_UNAVAILABLE", message: "Usage is unavailable or the request is invalid." } }, { status, headers: headersBase });
}
export async function proxyPlatformAIUsage(request: Request): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return unavailable(503);
  const url = new URL(request.url);
  const params = [...url.searchParams.entries()];
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (request.body || request.headers.get("content-length") && request.headers.get("content-length") !== "0" ||
    !generation || !/^[1-9]\d{0,18}$/.test(generation) || params.length > 6 ||
    new Set(params.map(([key]) => key)).size !== params.length || params.some(([key, value]) =>
      key === "limit" ? !/^(?:[1-9]|[1-9]\d|100)$/.test(value) : key === "cursor" ? !/^[A-Za-z0-9_-]{1,512}$/.test(value) : key === "tenant_id" ? !/^[a-f0-9-]{36}$/.test(value) : key === "kind" ? !["DOCUMENT_PDF", "DOCUMENT_DOCX", "GENERATION_TEXT"].includes(value) : key === "from" || key === "to" ? !/^\d{4}-\d{2}-\d{2}$/.test(value) || !Number.isFinite(Date.parse(`${value}T00:00:00Z`)) : true)) return unavailable(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/commercial/ai-usage${url.search}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return unavailable(503); }
  if (![200, 400, 401, 403, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return unavailable(503); }
  let body: ArrayBuffer;
  try { const bytes = await boundedUpstreamBody(upstream, 131_072); if (!bytes) return unavailable(503); body = bytes; } catch { return unavailable(503); }
  if (body.byteLength > 131_072) return unavailable(503);
  const responseHeaders = new Headers({ ...headersBase, "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
