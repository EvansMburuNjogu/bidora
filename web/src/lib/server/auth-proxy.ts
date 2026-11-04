import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { clientNetwork } from "@/lib/server/client-network";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
import { boundedUpstreamBody } from "@/lib/server/bounded-upstream-body";

const operations = {
  login: { path: "/v1/auth/login", body: true, cookie: false, csrf: false },
  register: { path: "/v1/auth/register", body: true, cookie: false, csrf: false },
  "password-recovery": { path: "/v1/auth/password-recovery", body: true, cookie: false, csrf: false },
  "password-reset": { path: "/v1/auth/password-recovery/reset", body: true, cookie: false, csrf: false },
  "verify-email": { path: "/v1/auth/verify-email", body: true, cookie: false, csrf: false },
  "verification-resend": { path: "/v1/auth/verify-email/resend", body: true, cookie: false, csrf: false },
  "login-code": { path: "/v1/auth/login-code/verify", body: true, cookie: true, csrf: false },
  "login-code-resend": { path: "/v1/auth/login-code/resend", body: true, cookie: true, csrf: false },
  "login-code-cancel": { path: "/v1/auth/login-code/cancel", body: false, cookie: true, csrf: false },
  "google-start": { path: "/v1/auth/google/start", body: false, cookie: false, csrf: false },
  logout: { path: "/v1/auth/logout", body: false, cookie: true, csrf: true },
  "password-change": { path: "/v1/auth/password", body: true, cookie: true, csrf: true },
  "mfa-enroll": { path: "/v1/auth/mfa/totp/enroll", body: true, cookie: true, csrf: true },
  "mfa-confirm": { path: "/v1/auth/mfa/totp/confirm", body: true, cookie: true, csrf: true },
  "mfa-step-up": { path: "/v1/auth/mfa/step-up", body: true, cookie: true, csrf: true },
  "mfa-recovery-codes": { path: "/v1/auth/mfa/recovery-codes/regenerate", body: true, cookie: true, csrf: true },
  "mfa-replace": { path: "/v1/auth/mfa/totp/replace", body: true, cookie: true, csrf: true },
} as const;

export type ProxyOperation = keyof typeof operations;

function googleFailureRedirect(headers?: Headers): Response {
  const responseHeaders = new Headers(headers);
  responseHeaders.set("Cache-Control", "private, no-store");
  responseHeaders.set("Location", "/auth/google/failed");
  responseHeaders.set("Referrer-Policy", "no-referrer");
  responseHeaders.set("X-Content-Type-Options", "nosniff");
  return new Response(null, { status: 303, headers: responseHeaders });
}

function clearGoogleCorrelation(headers: Headers) {
  headers.append(
    "Set-Cookie",
    "__Host-bidora_google_oidc=; Path=/; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:01 GMT; Secure; HttpOnly; SameSite=Lax",
  );
}

function safeFailure(status: 400 | 403 | 413 | 415 | 503): Response {
  const unavailable = status === 503;
  const forbidden = status === 403;
  const tooLarge = status === 413;
  const unsupported = status === 415;
  return Response.json(
    {
      error: {
        code: unavailable ? "AUTH_UNAVAILABLE" : forbidden ? "BROWSER_REQUEST_FORBIDDEN" : tooLarge ? "REQUEST_TOO_LARGE" : unsupported ? "UNSUPPORTED_MEDIA_TYPE" : "INVALID_REQUEST",
        message: unavailable
          ? "Authentication is temporarily unavailable."
          : forbidden
            ? "The browser request was rejected."
            : tooLarge
              ? "The request is too large."
              : unsupported
                ? "Content-Type must be application/json."
                : "The request is invalid.",
        field_errors: [],
        trace_id: crypto.randomUUID(),
        retryable: unavailable,
      },
    },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } },
  );
}

async function readBoundedBody(request: Request): Promise<ArrayBuffer | null> {
  const declared = Number(request.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > 4096) return null;
  if (!request.body) return null;

  const reader = request.body.getReader();
  const output = new Uint8Array(4096);
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (size + value.byteLength > output.byteLength) {
      await reader.cancel();
      return null;
    }
    output.set(value, size);
    size += value.byteLength;
  }
  return size === 0 ? null : output.buffer.slice(0, size);
}

export async function proxyAuth(request: Request, operation: ProxyOperation): Promise<Response> {
  const browser = webOrigin();
  const origin = apiOrigin();
  if (!browser || !origin) return operation === "google-start" ? googleFailureRedirect() : safeFailure(503);
  if (request.headers.get("origin") !== browser) return safeFailure(403);
  if (new URL(request.url).search) return safeFailure(400);

  const contract = operations[operation];
  const headers = new Headers({ Accept: "application/json", Origin: browser });
  const networkOperations = new Set<ProxyOperation>(["login", "login-code-resend", "register", "password-recovery", "password-change"]);
  if (networkOperations.has(operation)) {
    const network = clientNetwork(request);
    if (!network.ok) return safeFailure(503);
    if (network.address) headers.set("X-Forwarded-For", network.address);
  }
  let body: ArrayBuffer | undefined;

  if (contract.body) {
    if (request.headers.get("content-type")?.split(";")[0] !== "application/json") {
      return safeFailure(415);
    }
    body = await readBoundedBody(request) ?? undefined;
    if (!body) return safeFailure(413);
    headers.set("Content-Type", "application/json");
  } else if (request.body) {
    const reader = request.body.getReader();
    const first = await reader.read();
    await reader.cancel();
    if (!first.done && first.value.byteLength > 0) return safeFailure(400);
  }

  if (contract.cookie) {
    const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
    if (cookie) headers.set("Cookie", cookie);
  }
  if (contract.csrf) {
    const csrf = request.headers.get("x-csrf-token");
    if (!csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf)) return safeFailure(403);
    headers.set("X-CSRF-Token", csrf);
  }

  let upstream: Response;
  try {
    upstream = await fetch(origin + contract.path, {
      method: "POST",
      headers,
      body,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    return operation === "google-start" ? googleFailureRedirect() : safeFailure(503);
  }

  const responseHeaders = new Headers({
    "Cache-Control": "private, no-store",
    "X-Content-Type-Options": "nosniff",
  });
  for (const name of ["location", "retry-after", "referrer-policy", "x-request-id"]) {
    const value = upstream.headers.get(name);
    if (value) responseHeaders.set(name, value);
  }
  const challengeExpiry = upstream.headers.get("x-bidora-challenge-expires-at");
  if (upstream.status === 202 && (operation === "login" || operation === "login-code-resend") && challengeExpiry && Number.isFinite(Date.parse(challengeExpiry))) {
    responseHeaders.set("X-Bidora-Challenge-Expires-At", new Date(challengeExpiry).toISOString());
  }
  for (const value of upstream.headers.getSetCookie()) {
    responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
    if (upstream.status === 202 && (operation === "login" || operation === "login-code-resend") && value.startsWith("__Host-bidora_login_code=")) {
      const expiry = value.match(/;\s*Expires=([^;]+)/i)?.[1];
      if (!responseHeaders.has("X-Bidora-Challenge-Expires-At") && expiry && Number.isFinite(Date.parse(expiry))) responseHeaders.set("X-Bidora-Challenge-Expires-At", new Date(expiry).toISOString());
    }
  }

  if (upstream.status === 404) {
    await upstream.body?.cancel();
    return safeFailure(503);
  }

  if (operation === "google-start") {
    const location = upstream.headers.get("location");
    let valid = false;
    try {
      const url = location ? new URL(location) : null;
      valid = upstream.status === 303 && url?.origin === "https://accounts.google.com" && url.pathname === "/o/oauth2/v2/auth" && !!url.search && !url.username && !url.password && !url.hash;
    } catch { valid = false; }
    if (!valid) {
      await upstream.body?.cancel();
      return googleFailureRedirect(responseHeaders);
    }
  }

  // These commands deliberately return a generic, bodyless acknowledgement.
  // Fiber's SendStatus(202) can carry text/plain "Accepted", not JSON.
  const acknowledged = upstream.status === 202 && ["register", "password-recovery", "verification-resend", "login-code-resend"].includes(operation);
  if (acknowledged || upstream.status === 204 || (upstream.status >= 300 && upstream.status < 400)) {
    await upstream.body?.cancel();
    return new Response(null, { status: upstream.status, headers: responseHeaders });
  }

  const bytes = await boundedUpstreamBody(upstream);
  if (!bytes) return safeFailure(503);
  const contentType = upstream.headers.get("content-type");
  if (contentType?.split(";")[0] !== "application/json") return safeFailure(503);
  responseHeaders.set("Content-Type", "application/json");
  return new Response(bytes, { status: upstream.status, headers: responseHeaders });
}

export async function proxyGoogleCallback(request: Request): Promise<Response> {
  const origin = apiOrigin();
  const browser = webOrigin();
  const target = new URL(request.url);
  if (!origin || !browser || target.search.length > 8192) {
    const headers = new Headers();
    clearGoogleCorrelation(headers);
    return googleFailureRedirect(headers);
  }

  const headers = new Headers({ Accept: "application/json" });
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);

  let upstream: Response;
  try {
    upstream = await fetch(origin + "/v1/auth/google/callback" + target.search, {
      method: "GET",
      headers,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    const failureHeaders = new Headers();
    clearGoogleCorrelation(failureHeaders);
    return googleFailureRedirect(failureHeaders);
  }

  const responseHeaders = new Headers({
    "Cache-Control": "private, no-store",
    "Referrer-Policy": "no-referrer",
    "X-Content-Type-Options": "nosniff",
  });
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));
  const requestID = upstream.headers.get("x-request-id");
  if (requestID) responseHeaders.set("X-Request-ID", requestID);

  if (upstream.status !== 303) {
    await upstream.body?.cancel();
    return googleFailureRedirect(responseHeaders);
  }
  const location = upstream.headers.get("location");
  const allowedLocations = new Set([
    new URL("/auth/google/complete", browser).toString(),
    new URL("/auth/google/failed", browser).toString(),
  ]);
  if (!location || !allowedLocations.has(location)) {
    clearGoogleCorrelation(responseHeaders);
    return googleFailureRedirect(responseHeaders);
  }
  responseHeaders.set("Location", location);
  await upstream.body?.cancel();
  return new Response(null, { status: 303, headers: responseHeaders });
}
