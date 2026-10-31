import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
type Context = { params: Promise<{ key: string; operation: string }> };
export async function GET(request: Request, context: Context) { const { key, operation } = await context.params; return operation === "history" ? proxyManagedSettings(request, "templates", key, "history") : new Response(null, { status: 405 }); }
export async function POST(request: Request, context: Context) { const { key, operation } = await context.params; return operation === "preview" || operation === "test" ? proxyManagedSettings(request, "templates", key, operation) : new Response(null, { status: 405 }); }
