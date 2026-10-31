import { proxyPlatformAudit } from "@/lib/server/platform-audit-proxy";
export async function GET(request: Request, { params }: { params: Promise<{ auditId: string }> }) {
  const { auditId } = await params;
  return proxyPlatformAudit(request, auditId);
}
