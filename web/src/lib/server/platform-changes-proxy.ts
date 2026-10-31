import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const context = /^[1-9]\d{0,18}$/;

function failure(status: 400 | 403 | 503): Response {
  return Response.json({ error: { code: status === 503 ? "PLATFORM_UNAVAILABLE" : status === 403 ? "BROWSER_REQUEST_FORBIDDEN" : "INVALID_REQUEST", message: status === 503 ? "Platform administration is temporarily unavailable." : status === 403 ? "The browser request was rejected." : "The request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } }, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
}

export async function proxyAdministratorChanges(request: Request, operation: "list" | "create" | "approve" | "reject", changeID?: string): Promise<Response> {
  if ((operation === "approve" || operation === "reject") !== (changeID !== undefined) || (changeID && !uuid.test(changeID))) return failure(400);
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (!generation || !context.test(generation)) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  let body: ArrayBuffer | undefined;
  if (operation === "list") {
    if (request.body || Number(request.headers.get("content-length") ?? "0") !== 0) return failure(400);
    const entries = [...url.searchParams.entries()];
    if (entries.length < 1 || entries.length > 2 || url.searchParams.getAll("state").length !== 1 || url.searchParams.get("state") !== "PENDING_APPROVAL" ||
        url.searchParams.getAll("cursor").length > 1 || (url.searchParams.has("cursor") && !/^[A-Za-z0-9_-]{1,512}$/.test(url.searchParams.get("cursor") ?? "")) ||
        entries.some(([key]) => key !== "state" && key !== "cursor")) return failure(400);
  } else {
    if (request.headers.get("origin") !== browser) return failure(403);
    if (url.search || request.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(400);
    const csrf = request.headers.get("x-csrf-token");
    const key = request.headers.get("idempotency-key");
    const length = Number(request.headers.get("content-length"));
    if (!csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !key || !uuid.test(key) || !Number.isFinite(length) || length < 2 || length > 4096) return failure(400);
    try { body = await request.arrayBuffer(); } catch { return failure(400); }
    if (body.byteLength < 2 || body.byteLength > 4096) return failure(400);
    headers.set("Content-Type", "application/json");
    headers.set("X-CSRF-Token", csrf);
    headers.set("Idempotency-Key", key);
  }
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  const path = operation === "list" ? `/v1/platform/administrator-changes${url.search}` : operation === "create" ? "/v1/platform/administrator-changes" : `/v1/platform/administrator-changes/${changeID}/${operation}`;
  let upstream: Response;
  try { upstream = await fetch(backend + path, { method: operation === "list" ? "GET" : "POST", headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 201, 400, 401, 403, 404, 409, 413, 415, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id", "idempotency-replayed"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  const bytes = new Uint8Array(await upstream.arrayBuffer());
  if (bytes.byteLength > 262_144) return failure(503);
  return new Response(bytes, { status: upstream.status, headers: responseHeaders });
}
