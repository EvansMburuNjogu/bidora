import { proxyGoogleCallback } from "@/lib/server/auth-proxy";

export async function GET(request: Request) {
  return proxyGoogleCallback(request);
}
