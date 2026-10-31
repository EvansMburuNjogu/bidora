import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function failure(status: 400 | 403 | 503): Response {
  const unavailable = status === 503;
  return Response.json(
    { error: { code: unavailable ? "SESSION_ADMINISTRATION_UNAVAILABLE" : status === 403 ? "BROWSER_REQUEST_FORBIDDEN" : "INVALID_REQUEST", message: unavailable ? "Session administration is temporarily unavailable." : status === 403 ? "The browser request was rejected." : "The request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: unavailable } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } },
  );
}

async function hasBody(request: Request): Promise<boolean> {
  const declared = Number(request.headers.get("content-length") ?? "0");
  if (!Number.isFinite(declared) || declared > 0) return true;
  if (!request.body) return false;
  const reader = request.body.getReader();
  const first = await reader.read();
  await reader.cancel();
  return !first.done && first.value.byteLength > 0;
}

async function proxyMutation(request: Request, path: string, method: "DELETE" | "POST", idempotency?: string): Promise<Response> {
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return failure(503);
  if (request.headers.get("origin") !== browser) return failure(403);
  if (new URL(request.url).search || await hasBody(request)) return failure(400);
  const csrf = request.headers.get("x-csrf-token");
  if (!csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf)) return failure(403);

  const headers = new Headers({ Accept: "application/json", Origin: browser, "X-CSRF-Token": csrf });
  const cookie = request.headers.get("cookie");
  if (cookie) headers.set("Cookie", cookie);
  if (idempotency) headers.set("Idempotency-Key", idempotency);

  let upstream: Response;
  try {
    upstream = await fetch(backend + path, { method, headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) });
  } catch {
    return failure(503);
  }
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  for (const name of ["content-type", "retry-after", "x-request-id", "idempotency-replayed"]) {
    const value = upstream.headers.get(name);
    if (value) responseHeaders.set(name, value);
  }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", value);
  if (upstream.status === 204) {
    await upstream.body?.cancel();
    return new Response(null, { status: 204, headers: responseHeaders });
  }
  const bytes = new Uint8Array(await upstream.arrayBuffer());
  if (bytes.byteLength > 65_536 || upstream.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(503);
  return new Response(bytes, { status: upstream.status, headers: responseHeaders });
}

export async function proxyRevokeSession(request: Request, sessionID: string): Promise<Response> {
  if (!uuid.test(sessionID)) return failure(400);
  const idempotency = request.headers.get("idempotency-key");
  if (!idempotency || !uuid.test(idempotency)) return failure(400);
  return proxyMutation(request, `/v1/auth/sessions/${sessionID}`, "DELETE", idempotency);
}

export async function proxyRevokeAllSessions(request: Request): Promise<Response> {
  return proxyMutation(request, "/v1/auth/sessions/revoke-all", "POST");
}
