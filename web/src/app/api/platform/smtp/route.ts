import { proxyPlatformSMTP } from "@/lib/server/platform-proxy";

export async function GET(request: Request) { return proxyPlatformSMTP(request, "active"); }
export async function POST(request: Request) { return proxyPlatformSMTP(request, "create"); }
