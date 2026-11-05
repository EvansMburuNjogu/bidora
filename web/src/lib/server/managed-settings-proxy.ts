import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

const responseBase = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" };
function failure(status: number): Response { return Response.json({ error: { code: status === 503 ? "SETTINGS_UNAVAILABLE" : "INVALID_REQUEST", message: status === 503 ? "Settings are temporarily unavailable." : "The request is invalid.", retryable: status === 503, field_errors: [], trace_id: crypto.randomUUID() } }, { status, headers: responseBase }); }
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
async function boundedBody(request: Request): Promise<Uint8Array | null> {
  if (!request.body) return null;
  const reader = request.body.getReader(); const chunks: Uint8Array[] = []; let size = 0;
  try { for (;;) { const part = await reader.read(); if (part.done) break; size += part.value.byteLength; if (size > 16_384) { await reader.cancel(); return null; } chunks.push(part.value); } }
  catch { return null; }
  const body = new Uint8Array(size); let offset = 0; for (const chunk of chunks) { body.set(chunk, offset); offset += chunk.length; } return body;
}
export async function proxyManagedSettings(request: Request, section: "google" | "templates" | "countries" | "operations" | "shared-countries" | "availability", key?: string, operation?: "history" | "preview" | "test"): Promise<Response> {
  const api = apiOrigin(), browser = webOrigin(); if (!api || !browser) return failure(503);
  if (new URL(request.url).search || key !== undefined && !(section === "countries" ? /^[A-Z]{2}$/ : /^[a-z][a-z0-9_.-]{0,99}$/).test(key) || (section === "google" || section === "operations" || (section === "shared-countries" || section === "availability")) && key !== undefined || section === "templates" && operation !== undefined && !key || section === "countries" && operation !== undefined && !key || (section === "shared-countries" || section === "availability") && operation !== undefined) return failure(400);
  const method = request.method;
  const regional = section === "countries" || section === "operations" || (section === "shared-countries" || section === "availability");
  if (method === "GET" ? operation === "preview" || operation === "test" : (section === "shared-countries" || section === "availability") ? true : method === "PUT" ? operation !== undefined || (section === "templates" || section === "countries") && !key : method === "POST" ? section === "countries" ? !!key || operation !== undefined : regional || operation !== "preview" && operation !== "test" : true) return failure(405);
  const generation = request.headers.get("x-bidora-platform-context-generation"); if ((section !== "shared-countries" && section !== "availability") && (!generation || !/^[1-9]\d{0,18}$/.test(generation))) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser }); if ((section !== "shared-countries" && section !== "availability") && generation) headers.set("X-Bidora-Platform-Context-Generation", generation);
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser); if (cookie) headers.set("Cookie", cookie);
  let body: Uint8Array | undefined;
  if (method === "GET") { if (request.body || Number(request.headers.get("content-length") ?? "0") !== 0) return failure(400); }
  else {
    if (request.headers.get("origin") !== browser) return failure(403);
    const csrf = request.headers.get("x-csrf-token"); if (!csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || request.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(400);
    const value = await boundedBody(request); if (!value || value.length < 2) return failure(400); body = value;
    try { const json: unknown = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)); if (!json || typeof json !== "object" || Array.isArray(json)) return failure(400); } catch { return failure(400); }
    headers.set("Content-Type", "application/json"); headers.set("X-CSRF-Token", csrf);
    if (operation !== "preview") {
      const version = request.headers.get("if-match"), idempotency = request.headers.get("idempotency-key"); if (!(method === "POST" && section === "countries") && (!version || !/^"[1-9]\d{0,18}"$/.test(version)) || !idempotency || !uuid.test(idempotency)) return failure(400);
      if (version) headers.set("If-Match", version); headers.set("Idempotency-Key", idempotency);
    }
  }
  const path = regional ? (section === "shared-countries" || section === "availability") ? section === "availability" ? "/v1/shared/platform/availability" : "/v1/shared/platform/countries" : section === "operations" ? `/v1/platform/operational-policy${operation ? `/${operation}` : ""}` : `/v1/platform/countries${key ? `/${key}` : ""}${operation ? `/${operation}` : ""}` : section === "google" ? `/v1/platform/settings/google${operation ? `/${operation}` : ""}` : `/v1/platform/notification-templates${key ? `/${encodeURIComponent(key)}` : ""}${operation ? `/${operation}` : ""}`;
  let upstream: Response;
  try { upstream = await fetch(api + path, { method, headers, body: body as BodyInit | undefined, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(20_000) }); } catch { return failure(503); }
  if (![200, 400, 401, 403, 404, 409, 413, 415, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let result: ArrayBuffer; try { result = await upstream.arrayBuffer(); } catch { return failure(503); } if (result.byteLength > 1_048_576) return failure(503);
  const output = new Headers({ ...responseBase, "Content-Type": "application/json" });
  for (const name of ["x-request-id", "retry-after", "etag"]) { const value = upstream.headers.get(name); if (value) output.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) output.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(result, { status: upstream.status, headers: output });
}
