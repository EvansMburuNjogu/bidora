import { proxyPlatformInformation } from "@/lib/server/platform-information-proxy";
export async function GET(request: Request) { return proxyPlatformInformation(request, "security/session-policy"); }
export async function PUT(request: Request) { return proxyPlatformInformation(request, "security/session-policy"); }
