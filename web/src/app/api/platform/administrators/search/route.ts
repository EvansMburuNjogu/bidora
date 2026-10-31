import { proxyAdministratorEmailSearch } from "@/lib/server/platform-proxy";

export async function POST(request: Request) { return proxyAdministratorEmailSearch(request); }
