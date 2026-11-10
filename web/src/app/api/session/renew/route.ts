import { proxySessionRenew } from "@/lib/server/session-renew-proxy";
export async function POST(request: Request) { return proxySessionRenew(request); }
