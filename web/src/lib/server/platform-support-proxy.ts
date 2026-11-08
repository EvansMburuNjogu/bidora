import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const base = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" };
function failure(status: 400 | 403 | 503): Response {
  return Response.json({ error: { code: status === 503 ? "SUPPORT_UNAVAILABLE" : "INVALID_REQUEST", message: "The request is invalid or unavailable." } }, { status, headers: base });
}
type Action = "assign" | "reply" | "resolve" | "reopen";
export async function proxyPlatformSupport(request: Request, caseID?: string, action?: Action): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  if (caseID !== undefined && !uuid.test(caseID) || action && !caseID) return failure(400);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation)) return failure(400);
  const url = new URL(request.url), post = request.method === "POST";
  if (!post && (request.method !== "GET" || request.body || request.headers.get("content-length") && request.headers.get("content-length") !== "0")) return failure(400);
  if (post && url.search || caseID && url.search || post && request.headers.get("origin") !== browser) return failure(post ? 403 : 400);
  const params = [...url.searchParams.entries()];
  if (!post && (params.length > 2 || new Set(params.map(([key]) => key)).size !== params.length || params.some(([key, value]) => key === "limit" ? !/^(?:[1-9]|[1-4]\d|50)$/.test(value) : key === "cursor" ? !/^[A-Za-z0-9_-]{1,384}$/.test(value) : true))) return failure(400);
  const csrf = request.headers.get("x-csrf-token"), key = request.headers.get("idempotency-key");
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  let body: ArrayBuffer | undefined;
  if (post) {
    if (request.headers.get("content-type")?.split(";")[0] !== "application/json" || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !key || !uuid.test(key)) return failure(400);
    const length = Number(request.headers.get("content-length") ?? "0");
    if (!Number.isFinite(length) || length > 4096) return failure(400);
    try { body = await request.arrayBuffer(); } catch { return failure(400); }
    if (body.byteLength < 2 || body.byteLength > 4096) return failure(400);
    let parsed: unknown;
    try { parsed = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)); } catch { return failure(400); }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return failure(400);
    const fields = parsed as Record<string, unknown>;
    const allowed = action === "assign" ? ["assignee_id", "expected_version"] : action === "reply" ? ["message", "expected_version"] : action ? ["expected_version"] : ["tenant_id", "title", "description"];
    if (Object.keys(fields).length !== allowed.length || Object.keys(fields).some((name) => !allowed.includes(name)) ||
      action === "assign" && (typeof fields.assignee_id !== "string" || !uuid.test(fields.assignee_id)) ||
      action === "reply" && (typeof fields.message !== "string" || fields.message.trim() !== fields.message || fields.message.length < 1 || fields.message.length > 2000) ||
      action && (!Number.isSafeInteger(fields.expected_version) || (fields.expected_version as number) < 1) ||
      !action && (typeof fields.tenant_id !== "string" || !uuid.test(fields.tenant_id) || typeof fields.title !== "string" || fields.title.trim() !== fields.title || fields.title.length < 3 || fields.title.length > 120 || typeof fields.description !== "string" || fields.description.trim() !== fields.description || fields.description.length < 3 || fields.description.length > 2000)) return failure(400);
    headers.set("Content-Type", "application/json"); headers.set("X-CSRF-Token", csrf); headers.set("Idempotency-Key", key);
  }
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/support/cases${caseID ? `/${caseID}${action ? `/${action}` : ""}` : url.search}`, { method: request.method, headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 201, 400, 401, 403, 404, 409, 413, 415, 503].includes(upstream.status) || !post && upstream.status === 404 && !caseID || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  const reader = upstream.body?.getReader(); if (!reader) return failure(503);
  const chunks: Uint8Array[] = []; let size = 0;
  try { while (true) { const { done, value } = await reader.read(); if (done) break; size += value.byteLength; if (size > 131_072) { await reader.cancel(); return failure(503); } chunks.push(value); } }
  catch { await reader.cancel().catch(() => {}); return failure(503); }
  const bytes = new Uint8Array(size); let offset = 0; for (const part of chunks) { bytes.set(part, offset); offset += part.byteLength; }
  const responseHeaders = new Headers({ ...base, "Content-Type": "application/json" });
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(bytes, { status: upstream.status, headers: responseHeaders });
}
