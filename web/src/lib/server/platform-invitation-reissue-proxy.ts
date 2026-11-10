import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
const uuidPattern = /^[a-f\d]{8}-[a-f\d]{4}-[1-8][a-f\d]{3}-[89ab][a-f\d]{3}-[a-f\d]{12}$/i;
function failure(status: 400 | 503) { return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "PLATFORM_UNAVAILABLE", message: "The request is invalid or invitation data is unavailable.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } }, { status, headers: { "Cache-Control": "private, no-store", Vary: "Cookie, Origin" } }); }
export async function proxyInvitationRetry(request: Request, invitationID?: string): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin(), generation = request.headers.get("x-bidora-platform-context-generation");
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url), cursor = url.searchParams.get("cursor");
  const removing = invitationID !== undefined && request.method === "DELETE";
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation) || (invitationID ? !uuidPattern.test(invitationID) || !["POST", "DELETE"].includes(request.method) || url.search !== "" : request.method !== "GET" || request.body !== null || [...url.searchParams.keys()].some((key) => key !== "cursor") || url.searchParams.getAll("cursor").length > 1 || cursor !== null && !/^[A-Za-z0-9_-]{1,512}$/.test(cursor))) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  let body: ArrayBuffer | undefined;
  if (invitationID) {
    const csrf = request.headers.get("x-csrf-token"), key = request.headers.get("idempotency-key");
    const length = Number(request.headers.get("content-length") ?? "0");
    if (request.headers.get("origin") !== browser || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !key || !uuidPattern.test(key) || !Number.isFinite(length) || length < 0 || length > 2048) return failure(400);
    if (removing) {
      const version = request.headers.get("if-match");
      if (!version || !/^"[1-9][0-9]{0,18}"$/.test(version) || length !== 0) return failure(400);
      // Next.js may provide an empty stream for a bodyless DELETE request.
      const reader = request.body?.getReader();
      if (reader) {
        try {
          for (;;) { const chunk = await reader.read(); if (chunk.done) break; if (chunk.value.byteLength) { await reader.cancel(); return failure(400); } }
        } catch { return failure(400); }
        finally { reader.releaseLock(); }
      }
      headers.set("If-Match", version);
    } else {
    if (request.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(400);
    try { body = await request.arrayBuffer(); if (body.byteLength < 2 || body.byteLength > 2048) return failure(400); const value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)); if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).length !== 2 || !Number.isSafeInteger(value.expected_version) || value.expected_version < 1 || typeof value.reason !== "string" || value.reason !== value.reason.trim() || value.reason.length < 1 || value.reason.length > 2000) return failure(400); } catch { return failure(400); }
    headers.set("Content-Type", "application/json");
    }
    headers.set("X-CSRF-Token", csrf); headers.set("Idempotency-Key", key);
  }
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser); if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/administrator-invitations${invitationID ? `/${invitationID}${removing ? "" : "/reissue"}` : `?limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`}`, { method: request.method, headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(15_000) }); } catch { return failure(503); }
  if (removing && upstream.status === 204) {
    await upstream.body?.cancel();
    const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
    for (const name of ["x-request-id", "idempotency-replayed"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
    for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
    return new Response(null, { status: 204, headers: responseHeaders });
  }
  if (![200, 201, 400, 401, 403, 404, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let bytes: ArrayBuffer; try { bytes = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (bytes.byteLength > 65_536) return failure(503);
  const responseHeaders = new Headers({ "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  for (const name of ["retry-after", "x-request-id", "idempotency-replayed"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(bytes, { status: upstream.status, headers: responseHeaders });
}
