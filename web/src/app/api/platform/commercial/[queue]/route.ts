import { proxyPlatformCommercial } from "@/lib/server/platform-commercial-proxy";
export async function GET(request: Request, { params }: { params: Promise<{ queue: string }> }) {
  const { queue } = await params;
  return proxyPlatformCommercial(request, queue);
}
