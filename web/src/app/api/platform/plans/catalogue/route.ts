import { proxyPlatformPlanCatalogue } from "@/lib/server/platform-plan-catalogue-proxy";

export async function GET(request: Request) { return proxyPlatformPlanCatalogue(request); }
