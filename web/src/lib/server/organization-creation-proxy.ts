import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";

function safeFailure(status: 400 | 403 | 413 | 415 | 503): Response {
  const messages = {
    400: ["INVALID_REQUEST", "The request is invalid."],
    403: ["BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected."],
    413: ["REQUEST_TOO_LARGE", "The request is too large."],
    415: ["UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json."],
    503: ["ORGANIZATION_CREATION_UNAVAILABLE", "Organization creation is temporarily unavailable."],
  } as const;
  const [code, message] = messages[status];
  return Response.json(
    { error: { code, message, field_errors: [], trace_id: crypto.randomUUID(), retryable: status === 503 } },
    { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } },
  );
}

async function boundedBody(request: Request): Promise<{ body?: ArrayBuffer; tooLarge: boolean }> {
  const declared = Number(request.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > 4096) return { tooLarge: true };
  if (!request.body) return { tooLarge: false };
  const reader = request.body.getReader();
  const output = new Uint8Array(4096);
  let size = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (size + value.byteLength > output.byteLength) {
      await reader.cancel();
      return { tooLarge: true };
    }
    output.set(value, size);
    size += value.byteLength;
  }
  return size === 0
    ? { tooLarge: false }
    : { body: output.buffer.slice(0, size), tooLarge: false };
}

export async function proxyOrganizationCreation(request: Request): Promise<Response> {
  const backend = apiOrigin();
  const browser = webOrigin();
  if (!backend || !browser) return safeFailure(503);
  if (request.headers.get("origin") !== browser) return safeFailure(403);
  if (new URL(request.url).search) return safeFailure(400);
  if (request.headers.get("content-type")?.split(";")[0] !== "application/json") return safeFailure(415);

  const csrf = request.headers.get("x-csrf-token");
  if (!csrf || !/^[A-Za-z0-9_-]{43}$/.test(csrf)) return safeFailure(403);
  const idempotency = request.headers.get("idempotency-key");
  if (!idempotency || !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(idempotency)) {
    return safeFailure(400);
  }
  const bodyResult = await boundedBody(request);
  if (bodyResult.tooLarge) return safeFailure(413);
  if (!bodyResult.body) return safeFailure(400);

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
    upstream = await fetch(backend + "/v1/tenderpreneur/organizations", {
      method: "POST",
      headers,
      body: bodyResult.body,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    return safeFailure(503);
  }
  if (upstream.status === 404 || (upstream.status >= 500 && upstream.status !== 503)) {
    await upstream.body?.cancel();
    return safeFailure(503);
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
  const bytes = new Uint8Array(await upstream.arrayBuffer());
  if (bytes.byteLength > 65_536) return safeFailure(503);
  return new Response(bytes, { status: upstream.status, headers: responseHeaders });
}
