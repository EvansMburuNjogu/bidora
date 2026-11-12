import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
export async function GET(request: Request, context: RouteContext<"/api/session/organization-references/[kind]">) {
  const { kind } = await context.params;
  if (kind !== "business-types" && kind !== "industries") return new Response(null, { status: 404 });
  return proxyManagedSettings(request, kind === "business-types" ? "shared-business-types" : "shared-industries");
}
