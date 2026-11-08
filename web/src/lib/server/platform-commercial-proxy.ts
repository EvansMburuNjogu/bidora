import "server-only";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
import { commercialQueueConfig, commercialQueues, commercialUUID, type CommercialQueue } from "@/lib/platform-commercial-contract";

function failure(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "COMMERCIAL_UNAVAILABLE", message: "The financial view is unavailable or the request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
}
export async function proxyPlatformAdjustment(request: Request, changeID?: string): Promise<Response> {
  if (changeID !== undefined && !commercialUUID.test(changeID) || new URL(request.url).search || request.method !== "POST") return failure(400);
  const backend = apiOrigin(), browser = webOrigin(); if (!backend || !browser) return failure(503);
  const generation = request.headers.get("x-bidora-platform-context-generation"), csrf = request.headers.get("x-csrf-token"), key = request.headers.get("idempotency-key");
  if (request.headers.get("origin") !== browser || request.headers.get("content-type")?.split(";")[0] !== "application/json" || !generation || !/^[1-9]\d{0,18}$/.test(generation) || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !key || !commercialUUID.test(key) || Number(request.headers.get("content-length") ?? "0") > 4096) return failure(400);
  let body: ArrayBuffer, value: unknown;
  try { body = await request.arrayBuffer(); value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)); } catch { return failure(400); }
  if (body.byteLength < 2 || body.byteLength > 4096 || !value || typeof value !== "object" || Array.isArray(value)) return failure(400);
  const fields = value as Record<string, unknown>;
  const positive = (v: unknown) => Number.isSafeInteger(v) && Number(v) > 0;
  const safeText = (v: unknown) => typeof v === "string" && v.trim().length > 0 && new TextEncoder().encode(v).byteLength <= 2000;
  if (changeID === undefined ?
    Object.keys(fields).length !== 8 || !["invoice_id","action","reason_code","amount_minor_units","entitlement_disposition","proposal_reason","expected_invoice_version","expected_payment_version"].every((k) => Object.hasOwn(fields,k)) ||
    !commercialUUID.test(String(fields.invoice_id)) || !["ISSUE_CREDIT","REQUEST_REFUND"].includes(String(fields.action)) || !["SERVICE_CREDIT","DUPLICATE_PAYMENT_REFUND","SERVICE_FAILURE_REFUND","PAYMENT_REVERSAL_CORRECTION","OTHER_CONTRACTUAL_REFUND"].includes(String(fields.reason_code)) || !positive(fields.amount_minor_units) || !["NO_ENTITLEMENT_CHANGE","REDUCE_ADDITIONAL_LICENCES_AT_RENEWAL","END_PAID_ACCESS_ON_REFUND_SETTLEMENT","REVIEW_REQUIRED"].includes(String(fields.entitlement_disposition)) || fields.action === "ISSUE_CREDIT" && fields.entitlement_disposition !== "NO_ENTITLEMENT_CHANGE" || fields.action === "REQUEST_REFUND" && fields.entitlement_disposition === "NO_ENTITLEMENT_CHANGE" || !safeText(fields.proposal_reason) || !positive(fields.expected_invoice_version) || !positive(fields.expected_payment_version)
   : Object.keys(fields).length !== 3 || !["decision","reason","expected_version"].every((k) => Object.hasOwn(fields,k)) || !["APPROVED","REJECTED"].includes(String(fields.decision)) || !safeText(fields.reason) || !positive(fields.expected_version)) return failure(400);
  const headers = new Headers({ Accept: "application/json", "Content-Type": "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation, "X-CSRF-Token": csrf, "Idempotency-Key": key });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser); if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/commercial/financial-adjustments${changeID ? `/${changeID}/decision` : ""}`, { method: "POST", headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(15_000) }); } catch { return failure(503); }
  if (![changeID ? 200 : 201,400,401,403,404,409,503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let output: ArrayBuffer; try { output = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (output.byteLength > 65_536) return failure(503);
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const header = upstream.headers.get(name); if (header) responseHeaders.set(name, header); }
  for (const header of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(header, browser));
  return new Response(output, { status: upstream.status, headers: responseHeaders });
}
export async function proxyPaymentReconciliation(request: Request, paymentID: string): Promise<Response> {
  if (!commercialUUID.test(paymentID) || new URL(request.url).search) return failure(400);
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  if (request.headers.get("origin") !== browser || request.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(400);
  const generation = request.headers.get("x-bidora-platform-context-generation"), csrf = request.headers.get("x-csrf-token"), key = request.headers.get("idempotency-key");
  if (!generation || !/^[1-9]\d{0,18}$/.test(generation) || !csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf) || !key || !commercialUUID.test(key)) return failure(400);
  const length = Number(request.headers.get("content-length") ?? "0");
  if (!Number.isFinite(length) || length > 4096) return failure(400);
  let body: ArrayBuffer, value: unknown;
  try { body = await request.arrayBuffer(); value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body)); } catch { return failure(400); }
  if (body.byteLength < 2 || body.byteLength > 4096 || !value || typeof value !== "object" || Array.isArray(value)) return failure(400);
  const fields = value as Record<string, unknown>;
  if (Object.keys(fields).length !== 2 || !Object.hasOwn(fields, "expected_version") || !Object.hasOwn(fields, "reason") ||
      !Number.isSafeInteger(fields.expected_version) || Number(fields.expected_version) < 1 || typeof fields.reason !== "string" || !fields.reason.trim() || new TextEncoder().encode(fields.reason).byteLength > 2000) return failure(400);
  const headers = new Headers({ Accept: "application/json", "Content-Type": "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation, "X-CSRF-Token": csrf, "Idempotency-Key": key });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser); if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/commercial/payments/${paymentID}/reconciliation`, { method: "POST", headers, body, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(15_000) }); } catch { return failure(503); }
  if (![202, 400, 401, 403, 404, 409, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let output: ArrayBuffer; try { output = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (output.byteLength > 65_536) return failure(503);
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const header = upstream.headers.get(name); if (header) responseHeaders.set(name, header); }
  for (const header of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(header, browser));
  return new Response(output, { status: upstream.status, headers: responseHeaders });
}

export async function proxyPlatformCommercial(request: Request, queue: string, recordID?: string): Promise<Response> {
  if (!commercialQueues.includes(queue as CommercialQueue) || recordID !== undefined &&
      (!(queue === "financial-adjustments" || queue === "payment-reversals") || !commercialUUID.test(recordID))) return failure(400);
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const url = new URL(request.url);
  const length = Number(request.headers.get("content-length") ?? "0");
  const generation = request.headers.get("x-bidora-platform-context-generation");
  if (!Number.isFinite(length) || length !== 0 || request.body || !generation || !/^[1-9]\d{0,18}$/.test(generation)) return failure(400);
  const config = commercialQueueConfig[queue as CommercialQueue];
  const secondaryName = queue === "payments" ? "reconciliation_state" : queue === "financial-adjustments" ? "action" : "";
  const entries = [...url.searchParams.entries()];
  if (recordID ? entries.length !== 0 : entries.length > 4 || new Set(entries.map(([name]) => name)).size !== entries.length || entries.some(([name, value]) =>
    name === "cursor" ? !/^[A-Za-z0-9_-]{1,512}$/.test(value) : name === "limit" ? !/^(?:[1-9]|[1-9]\d|100)$/.test(value) :
      name === "state" ? !config.states.some((s) => s === value) : name === secondaryName && secondaryName ? !config.secondary.some((s) => s === value) : true)) return failure(400);
  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-Bidora-Platform-Context-Generation": generation });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  const path = recordID ? `/${queue}/${recordID}` : `/${queue}${url.search}`;
  // A reversal detail is addressed by payment_id; its queue id is not a detail locator.
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/platform/commercial${path}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 400, 401, 403, 404, 409, 503].includes(upstream.status) || upstream.status === 404 && !recordID ||
      upstream.headers.get("content-type")?.split(";")[0] !== "application/json") { await upstream.body?.cancel(); return failure(503); }
  let body: ArrayBuffer;
  try { body = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (body.byteLength > 262_144) return failure(503);
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
