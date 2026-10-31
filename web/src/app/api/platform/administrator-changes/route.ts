import { proxyAdministratorChanges } from "@/lib/server/platform-changes-proxy";

export async function GET(request: Request) { return proxyAdministratorChanges(request, "list"); }
export async function POST(request: Request) { return proxyAdministratorChanges(request, "create"); }
