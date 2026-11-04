import { proxyAuth, type ProxyOperation } from "@/lib/server/auth-proxy";

const allowed = new Set<ProxyOperation>([
  "login",
  "register",
  "password-recovery",
  "password-reset",
  "verify-email",
  "verification-resend",
  "login-code",
  "login-code-resend",
  "google-start",
  "logout",
  "password-change",
]);

export async function POST(request: Request, context: RouteContext<"/api/auth/[operation]">) {
  const { operation } = await context.params;
  if (!allowed.has(operation as ProxyOperation)) {
    return Response.json(
      { error: { code: "NOT_FOUND", message: "The requested resource was not found.", field_errors: [], trace_id: crypto.randomUUID(), retryable: false } },
      { status: 404, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", Vary: "Cookie, Origin" } },
    );
  }
  return proxyAuth(request, operation as ProxyOperation);
}
