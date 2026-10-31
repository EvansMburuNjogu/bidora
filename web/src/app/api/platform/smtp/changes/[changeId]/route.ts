import { proxySMTPRead } from "@/lib/server/platform-smtp-read-proxy";
export async function GET(request: Request, { params }: { params: Promise<{ changeId: string }> }) {
  const { changeId } = await params;
  return proxySMTPRead(request, changeId);
}
