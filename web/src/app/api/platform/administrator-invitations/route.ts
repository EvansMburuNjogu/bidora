import { proxyPlatformInvitation } from "@/lib/server/platform-proxy";
import { proxyInvitationRetry } from "@/lib/server/platform-invitation-reissue-proxy";
export async function GET(request: Request): Promise<Response> { return proxyInvitationRetry(request); }

export async function POST(request: Request): Promise<Response> {
  return proxyPlatformInvitation(request);
}
