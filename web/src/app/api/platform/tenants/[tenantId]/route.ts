import { proxyPlatformTenants } from "@/lib/server/platform-tenant-proxy";

export async function GET(request: Request, context: RouteContext<"/api/platform/tenants/[tenantId]">) {
  const { tenantId } = await context.params;
  return proxyPlatformTenants(request, tenantId);
}
