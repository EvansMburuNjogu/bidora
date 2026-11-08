import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

function failure(status: 400 | 403 | 503): Response {
  return Response.json(
    { error: { code: status === 403 ? "BROWSER_REQUEST_FORBIDDEN" : status === 503 ? "AUTH_UNAVAILABLE" : "INVALID_REQUEST", message: "The request could not be completed.", field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } },
  );
}

function cleared(browser: string): Response {
  const headers = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" });
  headers.append("Set-Cookie", browserSetCookie("__Host-bidora_login_code=; Path=/; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:01 GMT; Secure; HttpOnly; SameSite=Lax", browser));
  return new Response(null, { status: 204, headers });
}

export async function POST(request: Request): Promise<Response> {
  const browser = webOrigin();
  const api = apiOrigin();
  if (!browser || !api) return failure(503);
  if (request.headers.get("origin") !== browser) return failure(403);
  if (new URL(request.url).search) return failure(400);
  if (request.body) {
    const reader = request.body.getReader();
    const first = await reader.read();
    await reader.cancel();
    if (!first.done && first.value.byteLength > 0) return failure(400);
  }

  const headers = new Headers({ Accept: "application/json", Origin: browser });
  forwardAuditContext(request, headers);
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);

  let upstream: Response;
  try {
    upstream = await fetch(api + "/v1/auth/login-code/cancel", {
      method: "POST",
      headers,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    return failure(503);
  }

  await upstream.body?.cancel();
  if (upstream.status === 204 || upstream.status === 404) return cleared(browser);
  return failure(503);
}
