import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { clientNetwork } from "@/lib/server/client-network";

const googleAuthorizationOrigin = "https://accounts.google.com";
const googleAuthorizationPath = "/o/oauth2/v2/auth";

function safeFailure(status: 400 | 401 | 403 | 413 | 415 | 429 | 503): Response {
  const messages: Record<number, [string, string]> = {
    400: ["INVALID_REQUEST", "The request is invalid."],
    401: ["CREDENTIALS_INVALID", "The credentials are invalid."],
    403: ["BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected."],
    413: ["REQUEST_TOO_LARGE", "The request is too large."],
    415: ["UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json."],
    429: ["ATTEMPT_DENIED", "Authentication is temporarily unavailable."],
    503: ["GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable."],
  };
  const [code, message] = messages[status];
  return Response.json(
    { error: { code, message, field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 429 || status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "Referrer-Policy": "no-referrer", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } },
  );
}

async function boundedBody(request: Request): Promise<{ value: ArrayBuffer | null; tooLarge: boolean }> {
  const declared = Number(request.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > 4096) return { value: null, tooLarge: true };
  if (!request.body) return { value: null, tooLarge: false };
  const reader = request.body.getReader();
  const output = new Uint8Array(4096);
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (size + value.byteLength > output.length) {
      await reader.cancel();
      return { value: null, tooLarge: true };
    }
    output.set(value, size);
    size += value.byteLength;
  }
  return { value: size ? output.buffer.slice(0, size) : null, tooLarge: false };
}

function validGoogleAuthorization(value: string | null): value is string {
  if (!value) return false;
  try {
    const url = new URL(value);
    return url.origin === googleAuthorizationOrigin && url.pathname === googleAuthorizationPath && !!url.search && !url.username && !url.password && !url.hash;
  } catch {
    return false;
  }
}

function clearCorrelation(headers: Headers) {
  headers.append("Set-Cookie", "__Host-bidora_google_oidc=; Path=/; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:01 GMT; Secure; HttpOnly; SameSite=Lax");
}

export async function proxyGoogleLinkStart(request: Request): Promise<Response> {
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return safeFailure(503);
  if (request.headers.get("origin") !== browser) return safeFailure(403);
  if (new URL(request.url).search) return safeFailure(400);
  if (request.headers.get("content-type")?.split(";")[0] !== "application/json") return safeFailure(415);
  const csrf = request.headers.get("x-csrf-token");
  if (!csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf)) return safeFailure(403);
  const bounded = await boundedBody(request);
  if (bounded.tooLarge) return safeFailure(413);
  if (!bounded.value) return safeFailure(400);

  const network = clientNetwork(request);
  if (!network.ok) return safeFailure(503);
  const headers = new Headers({ Accept: "application/json", "Content-Type": "application/json", Origin: browser, "X-CSRF-Token": csrf });
  if (network.address) headers.set("X-Forwarded-For", network.address);
  const cookie = request.headers.get("cookie");
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try {
    upstream = await fetch(backend + "/v1/auth/google/link/start", { method: "POST", headers, body: bounded.value, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) });
  } catch {
    return safeFailure(503);
  }
  if (upstream.status !== 303) {
    const status = ([400, 401, 403, 413, 415, 429, 503] as const).find((value) => value === upstream.status) ?? 503;
    await upstream.body?.cancel();
    return safeFailure(status);
  }
  const authorizationURL = upstream.headers.get("location");
  if (!validGoogleAuthorization(authorizationURL)) {
    await upstream.body?.cancel();
    return safeFailure(503);
  }
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "Referrer-Policy": "no-referrer", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" });
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", value);
  const requestID = upstream.headers.get("x-request-id");
  if (requestID) responseHeaders.set("X-Request-ID", requestID);
  await upstream.body?.cancel();
  return Response.json({ authorization_url: authorizationURL }, { status: 200, headers: responseHeaders });
}

export async function proxyGoogleLinkCallback(request: Request): Promise<Response> {
  const backend = apiOrigin();
  const browser = webOrigin();
  const target = new URL(request.url);
  const failurePath = "/account/security/google-link-failed";
  if (!backend || !browser || target.search.length > 8192) {
    const responseHeaders = new Headers({ Location: failurePath, "Cache-Control": "private, no-store", "Referrer-Policy": "no-referrer" });
    clearCorrelation(responseHeaders);
    return new Response(null, { status: 303, headers: responseHeaders });
  }
  const headers = new Headers({ Accept: "application/json" });
  const cookie = request.headers.get("cookie");
  if (cookie) headers.set("Cookie", cookie);
  let upstream: Response;
  try {
    upstream = await fetch(backend + "/v1/auth/google/link/callback" + target.search, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: AbortSignal.timeout(10_000) });
  } catch {
    const responseHeaders = new Headers({ Location: failurePath, "Cache-Control": "private, no-store", "Referrer-Policy": "no-referrer" });
    clearCorrelation(responseHeaders);
    return new Response(null, { status: 303, headers: responseHeaders });
  }
  const responseHeaders = new Headers({ "Cache-Control": "private, no-store", "Referrer-Policy": "no-referrer", "X-Content-Type-Options": "nosniff" });
  const location = upstream.headers.get("location");
  const allowed = new Set([new URL("/account/security/google-linked", browser).toString(), new URL(failurePath, browser).toString()]);
  if (upstream.status !== 303 || !location || !allowed.has(location)) {
    clearCorrelation(responseHeaders);
    responseHeaders.set("Location", failurePath);
    await upstream.body?.cancel();
    return new Response(null, { status: 303, headers: responseHeaders });
  }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", value);
  const requestID = upstream.headers.get("x-request-id");
  if (requestID) responseHeaders.set("X-Request-ID", requestID);
  responseHeaders.set("Location", new URL(location).pathname);
  await upstream.body?.cancel();
  return new Response(null, { status: 303, headers: responseHeaders });
}
