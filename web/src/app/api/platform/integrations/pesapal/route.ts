import { proxyPlatformIntegration } from "@/lib/server/platform-proxy";
import { proxyIntegrationRead } from "@/lib/server/platform-smtp-read-proxy";
export async function POST(request: Request) { return proxyPlatformIntegration(request,"create","PESAPAL"); }
export async function GET(request: Request) { return proxyIntegrationRead(request, "PESAPAL", undefined, true); }
