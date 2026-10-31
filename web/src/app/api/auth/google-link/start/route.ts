import { proxyGoogleLinkStart } from "@/lib/server/google-link-proxy";

export async function POST(request: Request) {
  return proxyGoogleLinkStart(request);
}
