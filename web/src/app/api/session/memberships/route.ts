import { proxySessionRead } from "@/lib/server/session-proxy";

export async function GET(request: Request) {
  return proxySessionRead(request, "memberships");
}
