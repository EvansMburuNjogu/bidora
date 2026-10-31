import { proxyPlatformCommunications } from "@/lib/server/platform-communications-proxy";
export async function GET(request: Request) { return proxyPlatformCommunications(request); }
