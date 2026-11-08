import { proxyPlatformDashboard } from "@/lib/server/platform-dashboard-proxy";
export async function GET(request: Request) { return proxyPlatformDashboard(request, "overview"); }
