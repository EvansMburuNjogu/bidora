import { proxyPlatformAdministrators } from "@/lib/server/platform-proxy";

export async function GET(request: Request, context: RouteContext<"/api/platform/administrators/[userId]">) {
  const { userId } = await context.params;
  return proxyPlatformAdministrators(request, userId);
}
