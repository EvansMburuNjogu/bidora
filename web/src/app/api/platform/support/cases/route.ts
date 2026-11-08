import { proxyPlatformSupport } from "@/lib/server/platform-support-proxy";

export async function GET(request: Request): Promise<Response> { return proxyPlatformSupport(request); }
export async function POST(request: Request): Promise<Response> { return proxyPlatformSupport(request); }
