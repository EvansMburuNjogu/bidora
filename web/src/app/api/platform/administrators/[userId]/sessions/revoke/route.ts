import { proxyPlatformSessionRevocation } from "@/lib/server/platform-proxy";

export async function POST(request: Request, context: RouteContext<"/api/platform/administrators/[userId]/sessions/revoke">) {
  const { userId } = await context.params;
  return proxyPlatformSessionRevocation(request, userId);
}
