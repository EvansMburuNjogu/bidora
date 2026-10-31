import { proxyPlatformAdministrators } from "@/lib/server/platform-proxy";

export async function GET(request: Request, context: RouteContext<"/api/platform/administrators/[userId]/assignment-history">) {
  const { userId } = await context.params;
  return proxyPlatformAdministrators(request, userId, true);
}
