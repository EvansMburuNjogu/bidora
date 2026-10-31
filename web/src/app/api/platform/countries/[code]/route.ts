import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
export async function PUT(request: Request, context: { params: Promise<{code: string}> }) { const {code} = await context.params; return proxyManagedSettings(request, "countries", code); }
