import { proxyPlatformSubscription } from "@/lib/server/platform-subscription-proxy";
export async function GET(request: Request, context: { params: Promise<{ tenantId: string }> }) { const { tenantId } = await context.params; return proxyPlatformSubscription(request, tenantId, "read"); }
