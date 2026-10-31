import { proxyPlatformRoles } from "@/lib/server/platform-roles-proxy";
export async function GET(request: Request) { return proxyPlatformRoles(request); }
