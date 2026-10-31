import { proxyGoogleLinkCallback } from "@/lib/server/google-link-proxy";

export async function GET(request: Request) {
  return proxyGoogleLinkCallback(request);
}
