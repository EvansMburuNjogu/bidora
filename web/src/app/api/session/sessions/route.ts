import { proxySessionRead } from "@/lib/server/session-proxy";
import { proxyRevokeAllSessions } from "@/lib/server/session-administration-proxy";

export async function GET(request: Request) {
  return proxySessionRead(request, "sessions");
}

export async function POST(request: Request) {
  return proxyRevokeAllSessions(request);
}
