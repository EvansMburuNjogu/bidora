import { proxyPlatformSMTP } from "@/lib/server/platform-proxy";
export async function POST(request: Request) { return proxyPlatformSMTP(request, "approve"); }
