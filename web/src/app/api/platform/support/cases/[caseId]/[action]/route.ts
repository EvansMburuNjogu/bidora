import { proxyPlatformSupport } from "@/lib/server/platform-support-proxy";

const actions = ["assign", "reply", "resolve", "reopen"] as const;
export async function POST(request: Request, context: RouteContext<"/api/platform/support/cases/[caseId]/[action]">): Promise<Response> {
  const { caseId, action } = await context.params;
  if (!actions.some((name) => name === action)) return Response.json({ error: { code: "INVALID_REQUEST" } }, { status: 400, headers: { "Cache-Control": "private, no-store" } });
  return proxyPlatformSupport(request, caseId, action as typeof actions[number]);
}
