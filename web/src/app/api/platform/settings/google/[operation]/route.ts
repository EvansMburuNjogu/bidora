import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
type Context = { params: Promise<{ operation: string }> };
export async function GET(request: Request, context: Context) { return (await context.params).operation === "history" ? proxyManagedSettings(request, "google", undefined, "history") : new Response(null, { status: 405 }); }
export async function POST(request: Request, context: Context) { return (await context.params).operation === "test" ? proxyManagedSettings(request, "google", undefined, "test") : new Response(null, { status: 405 }); }
