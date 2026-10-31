import { proxyPlatformAdministrators } from "@/lib/server/platform-proxy";

export async function GET(request: Request) {
  return proxyPlatformAdministrators(request);
}
