import { proxyPlatformCommercial, proxyPlatformAdjustment } from "@/lib/server/platform-commercial-proxy";
export async function GET(request: Request, { params }: { params: Promise<{ queue: string }> }) {
  const { queue } = await params;
  return proxyPlatformCommercial(request, queue);
}
export async function POST(request: Request, { params }: { params: Promise<{ queue: string }> }) {
  const { queue } = await params;
  return queue === "financial-adjustments" ? proxyPlatformAdjustment(request) : Response.json({ error: { code: "INVALID_REQUEST", message: "The request is invalid.", field_errors: [], trace_id: crypto.randomUUID(), retryable: false } }, { status: 400, headers: { "Cache-Control": "private, no-store" } });
}
