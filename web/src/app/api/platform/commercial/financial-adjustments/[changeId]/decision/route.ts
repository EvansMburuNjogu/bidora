import { proxyPlatformAdjustment } from "@/lib/server/platform-commercial-proxy";
export async function POST(request: Request, { params }: { params: Promise<{ changeId: string }> }) {
  const { changeId } = await params;
  return proxyPlatformAdjustment(request, changeId);
}
