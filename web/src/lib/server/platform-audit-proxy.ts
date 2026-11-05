import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
import { auditActions, auditModules, auditResults, auditUUID } from "@/lib/platform-audit-contract";

const responseBase = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" };
function failure(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "AUDIT_UNAVAILABLE", message: "The audit view is unavailable or the request is invalid." } }, { status, headers: responseBase });
}
const date = (value: string) => /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(`${value}T00:00:00Z`)) && new Date(`${value}T00:00:00Z`).toISOString().slice(0, 10) === value;
export async function proxyPlatformAuditExport(request: Request): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url);
  const params = [...url.searchParams.entries()];
  const from = url.searchParams.get("from"), to = url.searchParams.get("to");
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (request.body || request.headers.get("content-length") && request.headers.get("content-length") !== "0" ||
    !generation || !/^[1-9]\d{0,18}$/.test(generation) || params.length !== 2 ||
    !from || !to || !date(from) || !date(to) || to < from ||
    (Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000 >= 31) return failure(400);
  const headers = new Headers({ Accept: "text/csv", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/audit/actions/export?${new URLSearchParams({ from, to })}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  const contentType = upstream.headers.get("content-type")?.split(";")[0];
  if (![200, 400, 401, 403, 409, 503].includes(upstream.status) || contentType !== (upstream.status === 200 ? "text/csv" : "application/json")) { await upstream.body?.cancel(); return failure(503); }
  const reader = upstream.body?.getReader();
  if (!reader) return failure(503);
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > 131_072) { await reader.cancel(); return failure(503); }
      chunks.push(value);
    }
  } catch { await reader.cancel().catch(() => {}); return failure(503); }
  const body = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) { body.set(chunk, offset); offset += chunk.byteLength; }
  const responseHeaders = new Headers({ ...responseBase, "Content-Type": upstream.status === 200 ? "text/csv; charset=utf-8" : "application/json" });
  if (upstream.status === 200) responseHeaders.set("Content-Disposition", `attachment; filename="platform-audit-${from}-${to}.csv"`);
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
export async function proxyPlatformAudit(request: Request, id?: string): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url);
  const params = [...url.searchParams.entries()];
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (request.body || request.headers.get("content-length") && request.headers.get("content-length") !== "0" ||
    !generation || !/^[1-9]\d{0,18}$/.test(generation) || id !== undefined && !auditUUID.test(id) ||
    id !== undefined && params.length !== 0 || params.length > 10 || new Set(params.map(([key]) => key)).size !== params.length ||
    params.some(([key, value]) => key === "limit" ? !/^(?:[1-9]|[1-9]\d|100)$/.test(value) :
      key === "cursor" ? !/^[A-Za-z0-9_-]{1,384}$/.test(value) :
      key === "search" ? value.length > 80 || value.trim() !== value || /[^\x20-\x7e]/.test(value) :
      key === "administrator" ? !auditUUID.test(value) :
      key === "action" ? !auditActions.some((action) => action === value) :
      key === "module" ? !auditModules.some((module) => module === value) :
      key === "result" ? !auditResults.some((result) => result === value) :
      key === "sort" ? value !== "ASC" && value !== "DESC" :
      key === "from" || key === "to" ? !date(value) : true) ||
    url.searchParams.has("from") && url.searchParams.has("to") && url.searchParams.get("from")! > url.searchParams.get("to")!) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/audit/actions${id ? `/${id}` : url.search}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 400, 401, 403, 404, 409, 503].includes(upstream.status) || upstream.status === 404 && id === undefined || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  const reader = upstream.body?.getReader();
  if (!reader) return failure(503);
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > (id ? 65_536 : 4_194_304)) { await reader.cancel(); return failure(503); }
      chunks.push(value);
    }
  } catch { await reader.cancel().catch(() => {}); return failure(503); }
  const body = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) { body.set(chunk, offset); offset += chunk.byteLength; }
  const responseHeaders = new Headers({ ...responseBase, "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
