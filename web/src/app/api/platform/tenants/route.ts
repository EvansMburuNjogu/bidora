import { proxyPlatformTenants } from "@/lib/server/platform-tenant-proxy";

export async function GET(request: Request) { return proxyPlatformTenants(request); }
