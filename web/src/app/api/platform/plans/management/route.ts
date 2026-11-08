import { proxyPlatformPlans } from "@/lib/server/platform-plan-proxy";
export async function GET(request: Request) { return proxyPlatformPlans(request, "management"); }
