import { proxyPlatformAuditExport } from "@/lib/server/platform-audit-proxy";

export async function GET(request: Request): Promise<Response> {
  return proxyPlatformAuditExport(request);
}
