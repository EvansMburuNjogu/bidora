import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";

function failure(status: 400 | 403 | 413 | 415 | 503): Response {
  const values = {
    400: ["INVALID_REQUEST", "The request is invalid."],
    403: ["BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected."],
    413: ["REQUEST_TOO_LARGE", "The request is too large."],
    415: ["UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json."],
    503: ["ACCOUNT_SETUP_UNAVAILABLE", "Account setup is temporarily unavailable."],
  } as const;
  const [code, message] = values[status];
  return Response.json(
    { error: { code, message, field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } },
  );
}

async function body(request: Request): Promise<{ value: ArrayBuffer | null; tooLarge: boolean }> {
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
  return { value: size === 0 ? null : output.buffer.slice(0, size), tooLarge: false };
}

export async function proxyAccountTypeSelection(request: Request): Promise<Response> {
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return failure(503);
  if (request.headers.get("origin") !== browser) return failure(403);
  if (new URL(request.url).search) return failure(400);
  if (request.headers.get("content-type")?.split(";")[0] !== "application/json") return failure(415);

  const csrf = request.headers.get("x-csrf-token");
  if (!csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf)) return failure(403);
  const idempotency = request.headers.get("idempotency-key");
  if (!idempotency || !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(idempotency)) return failure(400);
  const bounded = await body(request);
  if (bounded.tooLarge) return failure(413);
  if (!bounded.value) return failure(400);

  const headers = new Headers({
    Accept: "application/json",
    "Content-Type": "application/json",
    Origin: browser,
    "X-CSRF-Token": csrf,
    "Idempotency-Key": idempotency,
  });
  const cookie = request.headers.get("cookie");
  if (cookie) headers.set("Cookie", cookie);

  let upstream: Response;
  try {
    upstream = await fetch(backend + "/v1/me/account-type", {
      method: "POST",
      headers,
      body: bounded.value,
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
