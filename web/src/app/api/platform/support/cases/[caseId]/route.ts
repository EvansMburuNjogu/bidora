import { proxyPlatformSupport } from "@/lib/server/platform-support-proxy";

export async function GET(request: Request, context: RouteContext<"/api/platform/support/cases/[caseId]">): Promise<Response> {
  const { caseId } = await context.params;
  return proxyPlatformSupport(request, caseId);
}
