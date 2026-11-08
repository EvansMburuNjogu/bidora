import { proxyPaymentReconciliation } from "@/lib/server/platform-commercial-proxy";

export async function POST(request: Request, context: { params: Promise<{ paymentId: string }> }): Promise<Response> {
  return proxyPaymentReconciliation(request, (await context.params).paymentId);
}
