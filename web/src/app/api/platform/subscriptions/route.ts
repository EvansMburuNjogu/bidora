import { proxyPlatformSubscriptionInventory } from "@/lib/server/platform-subscription-proxy";
export async function GET(request: Request) { return proxyPlatformSubscriptionInventory(request); }
