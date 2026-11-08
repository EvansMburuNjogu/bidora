import { proxyPlatformPlans } from "@/lib/server/platform-plan-proxy";
export async function POST(request: Request, context: { params: Promise<{ key: string }> }) { const { key } = await context.params; return proxyPlatformPlans(request, "publish", key); }
