import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
type Context = { params: Promise<{ key: string }> };
export async function GET(request: Request, context: Context) { return proxyManagedSettings(request, "templates", (await context.params).key); }
export async function PUT(request: Request, context: Context) { return proxyManagedSettings(request, "templates", (await context.params).key); }
