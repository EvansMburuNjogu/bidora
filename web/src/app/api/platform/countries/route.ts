import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
export async function GET(request: Request) { return proxyManagedSettings(request, "countries"); }
export async function POST(request: Request) { return proxyManagedSettings(request, "countries"); }
