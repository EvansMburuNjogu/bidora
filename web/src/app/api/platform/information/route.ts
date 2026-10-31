import { proxyPlatformInformation } from "@/lib/server/platform-information-proxy";
export async function GET(request: Request) { return proxyPlatformInformation(request); }
export async function PUT(request: Request) { return proxyPlatformInformation(request); }
