import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
import { commercialQueueConfig, commercialQueues, commercialUUID, type CommercialQueue } from "@/lib/platform-commercial-contract";

function failure(status: 400 | 503): Response {
  return Response.json({ error: { code: status === 400 ? "INVALID_REQUEST" : "COMMERCIAL_UNAVAILABLE", message: "The financial view is unavailable or the request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
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
