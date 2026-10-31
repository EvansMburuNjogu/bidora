import { proxyPlatformAudit } from "@/lib/server/platform-audit-proxy";
export async function GET(request: Request) { return proxyPlatformAudit(request); }
