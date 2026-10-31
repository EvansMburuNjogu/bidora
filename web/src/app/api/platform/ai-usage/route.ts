import { proxyPlatformAIUsage } from "@/lib/server/platform-ai-usage-proxy";
export async function GET(request: Request) { return proxyPlatformAIUsage(request); }
