import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
export async function PUT(request: Request, context: RouteContext<"/api/platform/organization-references/[kind]/[code]">) {
  const { kind, code } = await context.params;
  if (kind !== "business-types" && kind !== "industries") return new Response(null, { status: 404 });
  return proxyManagedSettings(request, kind, code);
}
