import { proxyInvitationRetry } from "@/lib/server/platform-invitation-reissue-proxy";

export async function DELETE(request: Request, context: { params: Promise<{ invitationId: string }> }) {
  const { invitationId } = await context.params;
  return proxyInvitationRetry(request, invitationId);
}
