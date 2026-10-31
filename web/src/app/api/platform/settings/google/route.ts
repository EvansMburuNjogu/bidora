import { proxyManagedSettings } from "@/lib/server/managed-settings-proxy";
export async function GET(request: Request) { return proxyManagedSettings(request, "google"); }
export async function PUT(request: Request) { return proxyManagedSettings(request, "google"); }
