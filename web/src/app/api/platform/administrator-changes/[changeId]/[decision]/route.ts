import { proxyAdministratorChanges } from "@/lib/server/platform-changes-proxy";

export async function POST(request: Request, context: { params: Promise<{ changeId: string; decision: string }> }) {
  const { changeId, decision } = await context.params;
  if (decision !== "approve" && decision !== "reject") return new Response(null, { status: 404, headers: { "Cache-Control": "private, no-store" } });
  return proxyAdministratorChanges(request, decision, changeId);
}
