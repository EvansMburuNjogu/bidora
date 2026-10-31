import { proxyIntegrationRead } from "@/lib/server/platform-smtp-read-proxy";
export async function GET(request: Request) { return proxyIntegrationRead(request, "OPENAI"); }
