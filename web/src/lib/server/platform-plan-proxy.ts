import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

const planKey = /^(BASIC|GROWTH|PROFESSIONAL)$/;
const generationPattern = /^[1-9]\d{0,18}$/;
const uuidPattern = /^[a-f\d]{8}-[a-f\d]{4}-[1-8][a-f\d]{3}-[89ab][a-f\d]{3}-[a-f\d]{12}$/i;
function failed(status: 400 | 503) { return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "PLAN_UNAVAILABLE", message: "The request is invalid or plans are unavailable.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } }, { status, headers: { "Cache-Control": "private, no-store", Vary: "Cookie, Origin" } }); }
function positive(value: unknown): boolean { return Number.isSafeInteger(value) && Number(value) > 0; }
function validateBody(operation: "drafts" | "publish", value: unknown): boolean {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const data = value as Record<string, unknown>;
  const moneyFields = ["monthly_price", "annual_price", "additional_licence_monthly_price"];
  const numbers = ["bid_workspaces", "included_licences", "storage_bytes", "document_versions", "ai_credits", "concurrent_ai_jobs"];
  const fields = operation === "drafts" ? ["expected_head_version", ...moneyFields, ...numbers] : ["expected_head_version", "version"];
  if (Object.keys(data).length !== fields.length || !fields.every((field) => Object.hasOwn(data, field)) || !positive(data.expected_head_version)) return false;
  if (operation === "publish") return positive(data.version) && Number(data.version) >= 2;
  return numbers.every((field) => positive(data[field])) && moneyFields.every((field) => {
    const m = data[field]; if (!m || typeof m !== "object" || Array.isArray(m)) return false;
    const amount = m as Record<string, unknown>;
    return Object.keys(amount).length === 2 && amount.currency === "KES" && typeof amount.amount === "string" && /^(0|[1-9]\d{0,10})\.\d{2}$/.test(amount.amount) && Number(amount.amount) > 0;
  });
}
export async function proxyPlatformPlans(request: Request, operation: "management" | "drafts" | "publish", key?: string): Promise<Response> {
  const url = new URL(request.url), generation = request.headers.get("x-bidora-platform-context-generation"), browser = webOrigin(), backend = apiOrigin();
  if (!browser || !backend) return failed(503);
  if (url.search || !generation || !generationPattern.test(generation) || (operation === "management" ? key !== undefined || request.method !== "GET" || request.body !== null : !key || !planKey.test(key) || request.method !== "POST")) return failed(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  let body: ArrayBuffer | undefined;
  if (operation !== "management") {
    const csrf = request.headers.get("x-csrf-token"), idempotency = request.headers.get("idempotency-key");
    if (request.headers.get("origin") !== browser || request.headers.get("content-type")?.split(";")[0] !== "application/json" || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !idempotency || !uuidPattern.test(idempotency) || Number(request.headers.get("content-length") ?? "0") > 4096) return failed(400);
    try { body = await request.arrayBuffer(); if (body.byteLength < 2 || body.byteLength > 4096 || !validateBody(operation, JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)))) return failed(400); } catch { return failed(400); }
    headers.set("Content-Type", "application/json"); headers.set("X-CSRF-Token", csrf); headers.set("Idempotency-Key", idempotency);
  }
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser); if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/commercial/plans/${operation === "management" ? operation : `${key}/${operation}`}`, { method: request.method, headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(15_000) }); } catch { return failed(503); }
  if (![200, 201, 400, 401, 403, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failed(503); }
  let output: ArrayBuffer; try { output = await upstream.arrayBuffer(); } catch { return failed(503); }
  if (output.byteLength > 65_536) return failed(503);
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const cookieValue of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(cookieValue, browser));
  return new Response(output, { status: upstream.status, headers: responseHeaders });
}
