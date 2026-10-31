import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
import { proxySessionRead } from "@/lib/server/session-proxy";
import { parseMePortal } from "@/lib/session-contract";

function failure(status: 400 | 403 | 409 | 503): Response {
  return Response.json({ error: { code: status === 503 ? "PLATFORM_CATALOGUE_UNAVAILABLE" : status === 409 ? "PLATFORM_CONTEXT_STALE" : status === 403 ? "PLATFORM_CATALOGUE_FORBIDDEN" : "INVALID_REQUEST", message: "The plan catalogue is unavailable for this request.", retryable: status === 503, trace_id: crypto.randomUUID() } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } });
}

// The shared source catalogue is authenticated but not Platform-specific.
// Recheck the current Platform session before exposing it on a Platform route.
export async function proxyPlatformPlanCatalogue(request: Request): Promise<Response> {
  const backend = apiOrigin(), browser = webOrigin();
  if (!backend || !browser) return failure(503);
  const generation = request.headers.get("x-bidora-platform-context-generation");
  const length = Number(request.headers.get("content-length") ?? "0");
  if (new URL(request.url).search || request.body || !Number.isFinite(length) || length !== 0 || !generation || !/^[1-9]\d{0,18}$/.test(generation)) return failure(400);
  const session = await proxySessionRead(request, "me");
  if (!session.ok) return session;
  let me: ReturnType<typeof parseMePortal>;
  try { me = parseMePortal(await session.json()); } catch { return failure(503); }
  if (!me) return failure(503);
  if (me.data.platform_context_generation !== Number(generation)) return failure(409);
  if (me.data.account_type !== "PLATFORM_ADMIN" || me.data.portal?.path !== "/platform" ||
      me.data.session.assurance_level !== "AAL2" || !me.data.capabilities.includes("platform.plan.read")) return failure(403);

  const headers = new Headers({ Accept: "application/json", Origin: browser });
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try { upstream = await fetch(`${backend}/v1/shared/commercial/plans`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) }); }
  catch { return failure(503); }
  if (![200, 400, 401, 403, 503].includes(upstream.status) || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") {
    await upstream.body?.cancel(); return failure(503);
  }
  let body: ArrayBuffer;
  try { body = await upstream.arrayBuffer(); } catch { return failure(503); }
  if (body.byteLength > 65_536) return failure(503);
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin", "Content-Type": "application/json" });
  for (const name of ["retry-after", "x-request-id"]) { const value = upstream.headers.get(name); if (value) responseHeaders.set(name, value); }
  const cookies = upstream.headers.getSetCookie();
  for (const value of cookies.length ? cookies : session.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
