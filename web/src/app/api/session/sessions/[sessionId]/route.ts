import { proxyRevokeSession } from "@/lib/server/session-administration-proxy";

export async function DELETE(request: Request, context: RouteContext<"/api/session/sessions/[sessionId]">) {
  const { sessionId } = await context.params;
  return proxyRevokeSession(request, sessionId);
}
