import "server-only";
import { validSecurityControls } from "@/lib/platform-session-policy-contract";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

function failure(status: 400 | 403 | 503): Response {
  return Response.json({ error: { code: status === 503 ? "PLATFORM_UNAVAILABLE" : "INVALID_REQUEST", message: status === 503 ? "Platform settings are temporarily unavailable." : "The request is invalid.", retryable: status === 503 } }, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
}
export async function proxyPlatformInformation(request: Request, section: "information" | "security/session-policy" = "information"): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const method = request.method;
  if (method !== "GET" && method !== "PUT") return failure(400);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation) || new URL(request.url).search) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  let body: ArrayBuffer | undefined;
  if (method === "GET") {
    const length = Number(request.headers.get("content-length") ?? "0");
    if (!Number.isFinite(length) || length !== 0 || request.body) return failure(400);
  } else {
    if (request.headers.get("origin") !== browser) return failure(403);
    const length = Number(request.headers.get("content-length"));
    const csrf = request.headers.get("x-csrf-token"), version = request.headers.get("if-match"), key = request.headers.get("idempotency-key");
    if (!Number.isFinite(length) || length < 2 || length > 4096 || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !version || !/^"[1-9]\d*"$/.test(version) || !key || !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(key) || request.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(400);
    try { body = await request.arrayBuffer(); } catch { return failure(400); }
    if (body.byteLength < 2 || body.byteLength > 4096) return failure(400);
    let value: unknown;
    try { value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)); } catch { return failure(400); }
    if (!value || typeof value !== "object" || Array.isArray(value)) return failure(400);
    const fields = value as Record<string, unknown>;
    if (section === "security/session-policy") {
      if (Object.keys(fields).sort().join(",") !== "absolute_seconds,idle_seconds,security" || !Number.isSafeInteger(fields.idle_seconds) || !Number.isSafeInteger(fields.absolute_seconds) || !validSecurityControls(fields.security)) return failure(400);
    } else if (Object.keys(fields).length < 2 || Object.keys(fields).length > 4 || Object.keys(fields).some((key) => !["platform_name", "contact_phone", "support_email", "website"].includes(key)) || typeof fields.support_email !== "string" || typeof fields.website !== "string" || "platform_name" in fields && typeof fields.platform_name !== "string" || "contact_phone" in fields && typeof fields.contact_phone !== "string") return failure(400);
    headers.set("Content-Type", "application/json"); headers.set("X-CSRF-Token", csrf); headers.set("If-Match", version); headers.set("Idempotency-Key", key);
  }
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/${section}`, { method, headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); } catch { return failure(503); }
  if (![200, 400, 401, 403, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let result: ArrayBuffer;
  try { result = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (result.byteLength > 16_384) return failure(503);
  const responseHeaders = new Headers({ "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  const etag = upstream.headers.get("etag"); if (etag && /^"[1-9]\d*"$/.test(etag)) responseHeaders.set("ETag", etag);
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(result, { status: upstream.status, headers: responseHeaders });
}
