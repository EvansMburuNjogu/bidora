import { proxyPlatformSubscription } from "@/lib/server/platform-subscription-proxy";
export async function POST(request: Request, context: { params: Promise<{ tenantId: string }> }) { const { tenantId } = await context.params; return proxyPlatformSubscription(request, tenantId, "reactivate"); }
