import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";

const sessionReads = {
  bootstrap: "/v1/auth/session/bootstrap",
  me: "/v1/me",
  memberships: "/v1/me/memberships",
  sessions: "/v1/auth/sessions",
  "mfa-preparation": "/v1/auth/mfa/preparation",
  "platform-information": "/v1/shared/platform/information",
  "google-status": "/v1/auth/google/status",
} as const;

export type SessionRead = keyof typeof sessionReads;

function failure(status: 400 | 503): Response {
  const unavailable = status === 503;
  return Response.json(
    {
      error: {
        code: unavailable ? "AUTH_UNAVAILABLE" : "INVALID_REQUEST",
        message: unavailable ? "Authentication is temporarily unavailable." : "The request is invalid.",
        field_errors: [],
        trace_id: crypto.randomUUID(),
        retryable: unavailable,
      },
    },
    {
      status,
      headers: {
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
        Vary: "Cookie, Origin",
      },
    },
  );
}

export async function proxySessionRead(request: Request, operation: SessionRead): Promise<Response> {
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return failure(503);

  const url = new URL(request.url);
  const declaredLength = Number(request.headers.get("content-length") ?? "0");
  if (url.search || !Number.isFinite(declaredLength) || declaredLength > 0) return failure(400);

  const headers = new Headers({ Accept: "application/json", Origin: browser });
  const cookie = backendCookieHeader(request.headers.get("cookie"), browser);
  if (cookie) headers.set("Cookie", cookie);

  let upstream: Response;
  try {
    upstream = await fetch(backend + sessionReads[operation], {
      method: "GET",
      headers,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    return failure(503);
  }

  const responseHeaders = new Headers({
    "Cache-Control": "private, no-store",
    "X-Content-Type-Options": "nosniff",
    Vary: "Cookie, Origin",
  });
  for (const name of ["content-type", "retry-after", "x-request-id"]) {
    const value = upstream.headers.get(name);
    if (value) responseHeaders.set(name, value);
  }
  for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", browserSetCookie(value, browser));

  const body = new Uint8Array(await upstream.arrayBuffer());
  if (body.byteLength > 65_536) return failure(503);
  if (upstream.status !== 204 && upstream.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(503);
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}
