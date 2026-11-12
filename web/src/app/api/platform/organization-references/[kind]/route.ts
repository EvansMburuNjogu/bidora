import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
async function handle(request: Request, context: RouteContext<"/api/platform/organization-references/[kind]">) {
  const { kind } = await context.params;
  if (kind !== "business-types" && kind !== "industries") return new Response(null, { status: 404 });
  return proxyManagedSettings(request, kind);
}
export const GET = handle;
export const POST = handle;
